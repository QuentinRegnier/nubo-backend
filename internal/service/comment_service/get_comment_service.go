package comment_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetComments est la fonction hybride (ZSET -> Mongo -> Postgres) pour récupérer les commentaires.
// Elle renvoie désormais un tableau d'enveloppes (GetCommentOutput) pour gérer les erreurs partielles.
func GetComments(ctx context.Context, input comment_models.GetCommentsInput) ([]comment_models.GetCommentOutput, error) {
	var results []comment_models.GetCommentOutput

	// ─────────────────────────────────────────────────────────────────────────
	// 1. TENTATIVE L1 (VIP PARKING) : Le ZSET REDIS
	// ─────────────────────────────────────────────────────────────────────────

	if object_cache_service.IsPostInObjectCache(ctx, input.PostID) {

		ids, _ := object_cache_service.GetTopCommentIDs(ctx, input.PostID, input.Offset, input.Limit)

		if len(ids) > 0 {
			commentsMap := fetchCommentsCascade(ctx, ids)

			for _, id := range ids {
				c, ok := commentsMap[id]

				// Si introuvable ou Soft-Delete, on renvoie une erreur encapsulée pour cet ID
				if !ok || c.Visibility == -1 {
					results = append(results, comment_models.GetCommentOutput{
						CommentID: id,
						Error:     "Commentaire introuvable ou supprimé",
					})
				} else {
					// On copie la valeur pour avoir un pointeur sain
					val := c
					results = append(results, comment_models.GetCommentOutput{
						CommentID: id,
						Data:      &val,
					})
				}
			}
			return results, nil // ✅ RETOUR INSTANTANÉ
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. TENTATIVE L2 (PARKING LONGUE DURÉE) : MONGODB
	// ─────────────────────────────────────────────────────────────────────────
	comments, errMongo := mongo.MongoLoadCommentsPaginated(input.PostID, input.Offset, input.Limit)
	if errMongo == nil && len(comments) > 0 {
		for _, c := range comments {
			_ = object_cache_service.SetCommentInObjectCache(ctx, c)

			val := c
			results = append(results, comment_models.GetCommentOutput{
				CommentID: c.ID,
				Data:      &val,
			})
		}
		return results, nil // ✅ RETOUR RAPIDE
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 3. TENTATIVE L3 (LE GARAGE) : POSTGRESQL (Auto-Guérison L2 & L1)
	// ─────────────────────────────────────────────────────────────────────────
	if input.Offset == 0 {
		comments, errPg := postgres.FuncLoadCommentsPaginated(ctx, input.PostID, input.Offset, input.Limit)
		if errPg == nil {
			for _, c := range comments {
				_ = mongo.MongoUpsertComment(c)
				_ = object_cache_service.SetCommentInObjectCache(ctx, c)

				val := c
				results = append(results, comment_models.GetCommentOutput{
					CommentID: c.ID,
					Data:      &val,
				})
			}
			return results, nil
		}
	}

	return []comment_models.GetCommentOutput{}, nil
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
