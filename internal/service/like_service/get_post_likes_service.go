package like_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models" // ✅ NOUVEAU
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service" // ✅ NOUVEAU
)

// GetPostLikes récupère les abonnés ayant liké un post en appliquant les règles de visibilité.
func GetPostLikes(ctx context.Context, input like_models.GetPostLikesInput) (like_models.GetPostLikesOutput, error) {

	// ─────────────────────────────────────────────────────────────────────────
	// 1. SÉCURITÉ : VÉRIFICATION DES DROITS D'ACCÈS AU POST (L1 -> L2 -> L3)
	// ─────────────────────────────────────────────────────────────────────────
	var post post_models.PostPayload
	var found bool

	// Lecture ultra-rapide du post pour checker les droits
	if p, err := object_cache_service.GetPostFromObjectCache(ctx, input.PostID); err == nil {
		post = p
		found = true
	} else {
		// TENTATIVE L2 (MongoDB)
		mongoPosts, errMongo := mongo.MongoLoadPosts([]int64{input.PostID})
		if errMongo == nil && len(mongoPosts) > 0 {
			post = mongoPosts[0]
			found = true

			go func(p post_models.PostPayload) {
				_ = object_cache_service.SetPostInObjectCache(context.Background(), p)
			}(post)

		} else {
			// TENTATIVE L3 (PostgreSQL)
			pgPosts, errPg := postgres.FuncLoadPosts([]int64{input.PostID}, 1, 0)
			if errPg == nil && len(pgPosts) > 0 {
				post = pgPosts[0]
				found = true

				go func(p post_models.PostPayload) {
					_ = mongo.MongoUpsertPost(p)
					_ = object_cache_service.SetPostInObjectCache(context.Background(), p)
				}(post)
			}
		}
	}

	if !found || post.Visibility == -1 {
		return like_models.GetPostLikesOutput{}, nubo_error.NewNotFound("POST_NOT_FOUND", "Publication introuvable.", nil)
	}

	// Matrice de Confidentialité
	if post.UserID != input.CallerID {
		relationState := cache_service.RelationValue(ctx, post.UserID, input.CallerID)
		if relationState == -1 {
			return like_models.GetPostLikesOutput{}, nubo_error.NewForbidden("USER_BANNED", "Accès refusé.", nil)
		}
		if post.Visibility == 1 && relationState < 1 { // Abonnés
			return like_models.GetPostLikesOutput{}, nubo_error.NewForbidden("SUBSCRIBERS_ONLY", "Action non autorisée.", nil)
		}
		if post.Visibility == 2 && relationState != 2 { // Amis
			return like_models.GetPostLikesOutput{}, nubo_error.NewForbidden("FRIENDS_ONLY", "Action non autorisée.", nil)
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. RÉCUPÉRATION DES LIKES (L2 Mongo -> L3 Postgres)
	// ─────────────────────────────────────────────────────────────────────────
	var userIDs []int64

	// On tente Mongo (L2) d'abord
	userIDs, errMongo := mongo.MongoGetPostLikes(input.PostID, input.Limit, input.Offset)

	// Si Mongo échoue ou ne renvoie rien (Cache miss), on tape Postgres (L3)
	if errMongo != nil || len(userIDs) == 0 {
		userIDsPg, errPg := postgres.FuncGetPostLikes(ctx, input.PostID, input.Limit, input.Offset)
		if errPg == nil {
			userIDs = userIDsPg
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 3. HYDRATATION EN MASSE (SPEED CACHE + DOMAINE MÉDIA)
	// ─────────────────────────────────────────────────────────────────────────
	if len(userIDs) == 0 {
		return like_models.GetPostLikesOutput{
			PostID: input.PostID,
			Users:  make([]auth_models.UserLiteView, 0), // ✅ Tableau vide propre, pas de 'null' en JSON
		}, nil
	}

	var hydratedUsers []auth_models.UserLiteView

	for _, uID := range userIDs {
		// A. Récupération du pseudo et des métadonnées O(1) depuis le Speed Cache
		if userLite, err := cache_service.GetUserLite(ctx, uID); err == nil {

			// B. Signature HMAC de l'avatar si l'utilisateur en a un
			var avatarView media_models.MediaView
			if userLite.ProfilePictureID > 0 {
				// targetID = 0 (L'avatar n'est pas lié à un post spécifique), readerID = CallerID
				if view, errMedia := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, uID, 0, input.CallerID); errMedia == nil {
					avatarView = view
				}
			}

			// C. Assemblage de la UserLiteView
			hydratedUsers = append(hydratedUsers, auth_models.UserLiteView{
				User:     userLite,
				Avatar:   avatarView,
				IsOnline: cache_service.IsUserOnline(ctx, uID),
			})
		}
	}

	return like_models.GetPostLikesOutput{
		PostID: input.PostID,
		Users:  hydratedUsers, // ✅ Remplacé : La donnée est maintenant 100% exploitable
	}, nil
}
