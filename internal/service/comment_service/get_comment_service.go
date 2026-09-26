package comment_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DES COMMENTAIRES (CASCADE L1 -> L2 -> L3)
// ############################################################################

// GetComments est la fonction hybride pour récupérer les commentaires.
// Elle renvoie un tableau d'enveloppes (GetCommentOutput) pour isoler les erreurs partielles
// sans faire crasher la liste entière.
func GetComments(ctx context.Context, input comment_models.GetCommentsInput) ([]comment_models.GetCommentOutput, error) {
	var finalResults []comment_models.GetCommentOutput

	// ── ÉTAPE 0 : VÉRIFICATION DES DROITS D'ACCÈS AU POST PARENT ────────────

	var postPayload post_models.PostPayload
	postPayload, errCache := object_cache_service.GetPostFromObjectCache(ctx, input.PostID)

	if errCache != nil {
		// FALLBACK L2 (MongoDB) : On cherche le post dans le stockage tiède
		postsFromMongo, errMongo := mongo.MongoLoadPosts([]int64{input.PostID})
		if errMongo == nil && len(postsFromMongo) > 0 {
			postPayload = postsFromMongo[0]
			_ = object_cache_service.SetPostInObjectCache(ctx, postPayload) // Hydratation L1
		} else {
			// FALLBACK ABSOLU L3 (PostgreSQL)
			postsFromPostgres, errPg := postgres.FuncLoadPosts([]int64{input.PostID}, 1, 0)
			if errPg != nil {
				logger.Log.Error().Err(errPg).Int64("post_id", input.PostID).Msg("Erreur L3 lors de la vérification du post parent")
				return nil, nubo_error.NewInternal()
			}
			if len(postsFromPostgres) == 0 {
				return nil, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Le post parent est introuvable ou a été supprimé.", nil)
			}

			postPayload = postsFromPostgres[0]

			// AUTO-GUÉRISON EN CASCADE (L3 -> L2 -> L1)
			go func(p post_models.PostPayload) {
				bgCtx := context.Background()
				_ = object_cache_service.SetPostInObjectCache(bgCtx, p)
				_ = redis.EnqueueDB(bgCtx, p.ID, p.UserID, redis.EntityPost, redis.ActionUpdate, p, redis.TargetMongo)
			}(postPayload)
		}
	}

	// ⚡ MATRICE DE VISIBILITÉ EXACTE
	isAuthor := postPayload.UserID == input.UserID
	if !isAuthor {
		relationState := cache_service.RelationValue(ctx, postPayload.UserID, input.UserID)

		if postPayload.Visibility == variables.PostVisibilityDeleted || relationState == variables.RelationStateBlocked {
			return nil, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Accès refusé.", nil) // Bloqué ou Soft-Delete
		}
		if postPayload.Visibility == variables.PostVisibilitySubcriber && relationState < variables.RelationStateFollow {
			return nil, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Les commentaires sont réservés aux abonnés de cet utilisateur.", nil)
		}
		if postPayload.Visibility == variables.PostVisibilityFriend && relationState != variables.RelationStateFriend {
			return nil, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Les commentaires sont réservés aux amis de cet utilisateur.", nil)
		}
		if postPayload.Visibility == 3 {
			return nil, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Ce post est privé.", nil)
		}
	}

	// ── ÉTAPE 1 : TENTATIVE L1 (VIP PARKING ZSET REDIS) ─────────────────────

	// On n'utilise le ZSET L1 que pour les premiers commentaires (Offset faible)
	if input.Offset < variables.MaxZsetPostComment && object_cache_service.IsPostInObjectCache(ctx, input.PostID) {
		commentIDs, _ := object_cache_service.GetTopCommentIDs(ctx, input.PostID, input.Offset, input.Limit)

		if len(commentIDs) > 0 {
			hydratedCommentsMap := fetchCommentsCascade(ctx, commentIDs)

			for _, id := range commentIDs {
				commentPayload, exists := hydratedCommentsMap[id]

				// Si introuvable en base ou Soft-Delete, on renvoie une erreur encapsulée locale
				if !exists || commentPayload.Visibility == -1 {
					finalResults = append(finalResults, comment_models.GetCommentOutput{
						CommentID: id,
						Error:     "Ce commentaire est introuvable ou a été supprimé.",
					})
				} else {
					finalResults = append(finalResults, hydrateCommentOutput(ctx, input.UserID, commentPayload))
				}
			}
			return finalResults, nil // 🚀 RETOUR INSTANTANÉ L1
		}
	}

	// ── ÉTAPE 2 : TENTATIVE L2 (MONGODB) ────────────────────────────────────

	commentsFromMongo, errMongo := mongo.MongoLoadCommentsPaginated(input.PostID, input.Offset, input.Limit)
	if errMongo == nil && len(commentsFromMongo) > 0 {
		for _, commentPayload := range commentsFromMongo {
			_ = object_cache_service.SetCommentInObjectCache(ctx, commentPayload)

			if input.Offset < variables.MaxZsetPostComment {
				_ = object_cache_service.AddCommentToZSET(ctx, commentPayload.PostID, commentPayload.ID, float64(commentPayload.Score))
			}
			finalResults = append(finalResults, hydrateCommentOutput(ctx, input.UserID, commentPayload))
		}
		return finalResults, nil // 🚀 RETOUR RAPIDE L2
	}

	// ── ÉTAPE 3 : TENTATIVE L3 (POSTGRESQL - SOURCE DE VÉRITÉ) ──────────────

	if input.Offset == 0 {
		commentsFromPostgres, errPg := postgres.FuncLoadCommentsPaginated(ctx, input.PostID, input.Offset, input.Limit)
		if errPg != nil {
			logger.Log.Error().Err(errPg).Int64("post_id", input.PostID).Msg("Erreur L3 lors de la récupération des commentaires")
			return nil, nubo_error.NewInternal()
		}

		for _, commentPayload := range commentsFromPostgres {

			// AUTO-GUÉRISON L3 -> L2 & L1
			go func(c comment_models.CommentPayload) {
				bgCtx := context.Background()
				_ = object_cache_service.SetCommentInObjectCache(bgCtx, c)
				_ = redis.EnqueueDB(bgCtx, c.ID, c.PostID, redis.EntityComment, redis.ActionUpdate, c, redis.TargetMongo)
			}(commentPayload)

			_ = object_cache_service.AddCommentToZSET(ctx, commentPayload.PostID, commentPayload.ID, float64(commentPayload.Score))
			finalResults = append(finalResults, hydrateCommentOutput(ctx, input.UserID, commentPayload))
		}
		return finalResults, nil
	}

	// Aucun commentaire trouvé
	return []comment_models.GetCommentOutput{}, nil
}
