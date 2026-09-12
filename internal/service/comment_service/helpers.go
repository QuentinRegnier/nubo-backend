package comment_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// hydrateCommentOutput assemble l'output final du commentaire en récupérant l'avatar de son auteur
func hydrateCommentOutput(ctx context.Context, callerID int64, c comment_models.CommentPayload) comment_models.GetCommentOutput {
	var avatar media_models.MediaView // Zéro-valeur par défaut (pas de pointeur)
	var username string               // ✅ NOUVEAU : On prépare la variable pour le pseudo

	// Lecture ultra-rapide O(1) de l'empreinte de l'auteur dans le Speed Cache
	if authorLite, err := cache_service.GetUserLite(ctx, c.UserID); err == nil {

		username = authorLite.Username // ✅ NOUVEAU : On assigne le pseudo !

		if authorLite.ProfilePictureID > 0 {
			// Appel du domaine Média pour signer l'URL
			// targetID = 0 (pas de post spécifique lié à la photo de profil)
			if view, errMedia := media_service.GenerateMediaViewCascade(ctx, authorLite.ProfilePictureID, authorLite.ID, 0, callerID); errMedia == nil {
				avatar = view
			}
		}
	}

	return comment_models.GetCommentOutput{
		CommentID:      c.ID,
		Data:           c,
		AuthorUsername: username, // ✅ NOUVEAU : On passe le pseudo à la structure
		AuthorAvatar:   avatar,   // Composition par valeur
	}
}

// fetchCommentsCascade gère l'hydratation L1 -> L2 -> L3 pour un batch d'IDs
func fetchCommentsCascade(ctx context.Context, ids []int64) map[int64]comment_models.CommentPayload {
	commentsMap := make(map[int64]comment_models.CommentPayload)
	var missingFromL1 []int64

	// Étape 1 : Object Cache LFU (Redis)
	for _, id := range ids {
		if c, err := object_cache_service.GetCommentFromObjectCache(ctx, id); err == nil {
			commentsMap[id] = c
		} else {
			missingFromL1 = append(missingFromL1, id)
		}
	}

	if len(missingFromL1) == 0 {
		return commentsMap
	}

	// Étape 2 : Cold Storage (MongoDB)
	var missingFromL2 []int64
	mongoComments, errMongo := mongo.MongoLoadComments(missingFromL1)
	if errMongo == nil {
		for _, c := range mongoComments {
			commentsMap[c.ID] = c
			_ = object_cache_service.SetCommentInObjectCache(ctx, c)
		}
	}

	for _, id := range missingFromL1 {
		if _, exists := commentsMap[id]; !exists {
			missingFromL2 = append(missingFromL2, id)
		}
	}

	if len(missingFromL2) == 0 {
		return commentsMap
	}

	// Étape 3 : Base de Données (PostgreSQL)
	for _, id := range missingFromL2 {
		if c, err := postgres.FuncGetComment(ctx, id); err == nil {
			commentsMap[c.ID] = c
			_ = mongo.MongoUpsertComment(c)
			_ = object_cache_service.SetCommentInObjectCache(ctx, c)
		}
	}

	return commentsMap
}
