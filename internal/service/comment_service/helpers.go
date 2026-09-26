package comment_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// ############################################################################
// # HELPERS : FONCTIONS D'AIDE (HYDRATATION ET CASCADE)
// ############################################################################

// hydrateCommentOutput assemble l'output final du commentaire en récupérant le pseudo
// et l'avatar sécurisé (HMAC) de son auteur via le Speed Cache.
func hydrateCommentOutput(ctx context.Context, callerID int64, commentPayload comment_models.CommentPayload) comment_models.GetCommentOutput {
	var avatarView media_models.MediaView // Zéro-valeur par défaut (pas de pointeur)
	var authorUsername string

	// Lecture ultra-rapide O(1) de l'empreinte de l'auteur dans le Speed Cache
	if authorLite, errLite := cache_service.GetUserLite(ctx, commentPayload.UserID); errLite == nil {

		authorUsername = authorLite.Username

		if authorLite.ProfilePictureID > 0 {
			// Appel du domaine Média pour signer cryptographiquement l'URL (S3/MinIO)
			// targetID = 0 (pas de post spécifique lié à la photo de profil)
			if view, errMedia := media_service.GenerateMediaViewCascade(ctx, authorLite.ProfilePictureID, authorLite.ID, 0, callerID); errMedia == nil {
				avatarView = view
			}
		}
	}

	return comment_models.GetCommentOutput{
		CommentID:      commentPayload.ID,
		Data:           commentPayload,
		AuthorUsername: authorUsername,
		AuthorAvatar:   avatarView, // Composition par valeur
	}
}

// fetchCommentsCascade gère l'hydratation L1 -> L2 -> L3 pour un lot (batch) d'IDs.
// Il garantit que chaque commentaire manquant en RAM remonte depuis les disques froids.
func fetchCommentsCascade(ctx context.Context, commentIDs []int64) map[int64]comment_models.CommentPayload {
	commentsMap := make(map[int64]comment_models.CommentPayload)
	var missingFromL1 []int64

	// ── ÉTAPE 1 : OBJECT CACHE LFU (REDIS) ──────────────────────────────────

	for _, id := range commentIDs {
		if commentPayload, errL1 := object_cache_service.GetCommentFromObjectCache(ctx, id); errL1 == nil {
			commentsMap[id] = commentPayload
		} else {
			missingFromL1 = append(missingFromL1, id)
		}
	}

	if len(missingFromL1) == 0 {
		return commentsMap // Tout était en RAM !
	}

	// ── ÉTAPE 2 : WARM STORAGE (MONGODB) ────────────────────────────────────

	var missingFromL2 []int64
	commentsFromMongo, errMongo := mongo.MongoLoadComments(missingFromL1)

	if errMongo == nil {
		for _, commentPayload := range commentsFromMongo {
			commentsMap[commentPayload.ID] = commentPayload
			_ = object_cache_service.SetCommentInObjectCache(ctx, commentPayload) // Auto-guérison L1
		}
	} else if errMongo.Error() != "mongo: no documents in result" {
		logger.Log.Warn().Err(errMongo).Msg("Avertissement L2 Mongo lors du fetchCommentsCascade")
	}

	// Identification des restes
	for _, id := range missingFromL1 {
		if _, exists := commentsMap[id]; !exists {
			missingFromL2 = append(missingFromL2, id)
		}
	}

	if len(missingFromL2) == 0 {
		return commentsMap
	}

	// ── ÉTAPE 3 : COLD STORAGE (POSTGRESQL) ─────────────────────────────────

	for _, id := range missingFromL2 {
		if commentPayload, errPg := postgres.FuncGetComment(ctx, id); errPg == nil {
			commentsMap[commentPayload.ID] = commentPayload

			// AUTO-GUÉRISON L3 -> L2 & L1
			go func(c comment_models.CommentPayload) {
				bgCtx := context.Background()
				// L1 : Hydratation immédiate en RAM
				_ = object_cache_service.SetCommentInObjectCache(bgCtx, c)

				// L2 : Réplication asynchrone vers Mongo
				_ = redis.EnqueueDB(bgCtx, c.ID, c.PostID, redis.EntityComment, redis.ActionUpdate, c, redis.TargetMongo)
			}(commentPayload)
		} else {
			logger.Log.Error().Err(errPg).Int64("comment_id", id).Msg("Erreur L3 Postgres lors de la récupération unitaire d'un commentaire")
		}
	}

	return commentsMap
}
