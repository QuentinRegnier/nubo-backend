package comment_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetComments est la fonction hybride (ZSET -> Mongo -> Postgres) pour récupérer les commentaires.
// Elle renvoie désormais un tableau d'enveloppes (GetCommentOutput) pour gérer les erreurs partielles.
func GetComments(ctx context.Context, input comment_models.GetCommentsInput) ([]comment_models.GetCommentOutput, error) {
	var results []comment_models.GetCommentOutput

	// ─────────────────────────────────────────────────────────────────────────
	// 0. SÉCURITÉ : VÉRIFICATION DES DROITS D'ACCÈS AU POST PARENT
	// ─────────────────────────────────────────────────────────────────────────
	var post post_models.PostPayload
	post, err := object_cache_service.GetPostFromObjectCache(ctx, input.PostID)
	if err != nil {
		// Fallback L2 (MongoDB) : On cherche le post dans le stockage à froid
		mongoPosts, errMongo := mongo.MongoLoadPosts([]int64{input.PostID})
		if errMongo == nil && len(mongoPosts) > 0 {
			post = mongoPosts[0]
			_ = object_cache_service.SetPostInObjectCache(ctx, post) // Hydratation L1
		} else {
			// Fallback absolu L3 (PostgreSQL)
			pgPosts, errPg := postgres.FuncLoadPosts([]int64{input.PostID}, 1, 0)
			if errPg != nil || len(pgPosts) == 0 {
				return nil, nubo_error.NewNotFound("POST_NOT_FOUND", "Le post parent est introuvable ou a été supprimé.", errPg)
			}
			post = pgPosts[0]

			// HYDRATATION EN CASCADE COMPLÈTE (L3 -> L2 -> L1)
			go func(p post_models.PostPayload) {
				bgCtx := context.Background()
				_ = object_cache_service.SetPostInObjectCache(bgCtx, p)
				_ = redis.EnqueueDB(bgCtx, p.ID, p.UserID, redis.EntityPost, redis.ActionUpdate, p, redis.TargetMongo)
			}(post)
		}
	}

	// ⚡ MATRICE DE VISIBILITÉ EXACTE DE NUBO
	isAuthor := post.UserID == input.UserID
	if !isAuthor {
		relationState := cache_service.RelationValue(ctx, post.UserID, input.UserID)
		if relationState == -1 || post.Visibility == -1 {
			return nil, nubo_error.NewForbidden("ACCESS_DENIED", "Accès refusé.", nil) // Bloqué ou Supprimé
		}
		if post.Visibility == 1 && relationState < 1 {
			return nil, nubo_error.NewForbidden("SUBSCRIBERS_ONLY", "Les commentaires sont réservés aux abonnés.", nil)
		}
		if post.Visibility == 2 && relationState != 2 {
			return nil, nubo_error.NewForbidden("FRIENDS_ONLY", "Les commentaires sont réservés aux amis.", nil)
		}
		if post.Visibility == 3 {
			return nil, nubo_error.NewForbidden("PRIVATE_POST", "Post privé.", nil)
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 1. TENTATIVE L1 (VIP PARKING) : Le ZSET REDIS
	// ─────────────────────────────────────────────────────────────────────────

	if input.Offset < 100 && object_cache_service.IsPostInObjectCache(ctx, input.PostID) {
		ids, _ := object_cache_service.GetTopCommentIDs(ctx, input.PostID, input.Offset, input.Limit)
		if len(ids) > 0 {
			commentsMap := fetchCommentsCascade(ctx, ids)
			for _, id := range ids {
				c, ok := commentsMap[id]
				// Si introuvable ou Soft-Delete, on renvoie une erreur encapsulée
				if !ok || c.Visibility == -1 {
					results = append(results, comment_models.GetCommentOutput{
						CommentID: id,
						Error:     "Commentaire introuvable ou supprimé",
					})
				} else {
					// HYDRATATION DE L'AUTEUR
					results = append(results, hydrateCommentOutput(ctx, input.UserID, c))
				}
			}
			return results, nil // 🚀 RETOUR INSTANTANÉ
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. TENTATIVE L2 (PARKING LONGUE DURÉE) : MONGODB
	// ─────────────────────────────────────────────────────────────────────────
	comments, errMongo := mongo.MongoLoadCommentsPaginated(input.PostID, input.Offset, input.Limit)
	if errMongo == nil && len(comments) > 0 {
		for _, c := range comments {
			_ = object_cache_service.SetCommentInObjectCache(ctx, c)
			if input.Offset < 100 {
				_ = object_cache_service.AddCommentToZSET(ctx, c.PostID, c.ID, float64(c.Score))
			}
			// HYDRATATION DE L'AUTEUR
			results = append(results, hydrateCommentOutput(ctx, input.UserID, c))
		}
		return results, nil // 🚀 RETOUR RAPIDE
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 3. TENTATIVE L3 (LE GARAGE) : POSTGRESQL (Auto-Guérison L2 & L1)
	// ─────────────────────────────────────────────────────────────────────────
	if input.Offset == 0 {
		comments, errPg := postgres.FuncLoadCommentsPaginated(ctx, input.PostID, input.Offset, input.Limit)
		if errPg == nil {
			for _, c := range comments {
				// ⬆️ PROMOTION L3 -> L2 & L1
				go func(comment comment_models.CommentPayload) {
					bgCtx := context.Background()
					_ = object_cache_service.SetCommentInObjectCache(bgCtx, comment)
					_ = redis.EnqueueDB(bgCtx, comment.ID, comment.PostID, redis.EntityComment, redis.ActionUpdate, comment, redis.TargetMongo)
				}(c)

				_ = object_cache_service.AddCommentToZSET(ctx, c.PostID, c.ID, float64(c.Score))

				// HYDRATATION DE L'AUTEUR
				results = append(results, hydrateCommentOutput(ctx, input.UserID, c))
			}
			return results, nil
		}
	}

	return []comment_models.GetCommentOutput{}, nil
}
