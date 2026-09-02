package comment_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
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
			_ = mongo.MongoUpsertPost(post)
			_ = object_cache_service.SetPostInObjectCache(ctx, post)
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
				_ = mongo.MongoUpsertComment(c)
				_ = object_cache_service.SetCommentInObjectCache(ctx, c)
				_ = object_cache_service.AddCommentToZSET(ctx, c.PostID, c.ID, float64(c.Score))
				// HYDRATATION DE L'AUTEUR
				results = append(results, hydrateCommentOutput(ctx, input.UserID, c))
			}
			return results, nil
		}
	}

	return []comment_models.GetCommentOutput{}, nil
}

// -------------------------------------------------------------------------
// FONCTIONS UTILITAIRES PRIVÉES
// -------------------------------------------------------------------------

// hydrateCommentOutput assemble l'output final du commentaire en récupérant l'avatar de son auteur
func hydrateCommentOutput(ctx context.Context, callerID int64, c comment_models.CommentPayload) comment_models.GetCommentOutput {
	var avatar media_models.MediaView // Zéro-valeur par défaut (pas de pointeur)

	// Lecture ultra-rapide O(1) de l'empreinte de l'auteur dans le Speed Cache
	if authorLite, err := cache_service.GetUserLite(ctx, c.UserID); err == nil {
		if authorLite.ProfilePictureID > 0 {
			// Appel du domaine Média pour signer l'URL
			// targetID = 0 (pas de post spécifique lié à la photo de profil)
			if view, errMedia := media_service.GenerateMediaViewCascade(ctx, authorLite.ProfilePictureID, authorLite.ID, 0, callerID); errMedia == nil {
				avatar = view
			}
		}
	}

	return comment_models.GetCommentOutput{
		CommentID:    c.ID,
		Data:         c,
		AuthorAvatar: avatar, // Composition par valeur
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
