package post_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/comment_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// GetPosts orchestre la récupération d'une liste de posts et applique le filtrage de visibilité.
func GetPosts(ctx context.Context, input post_models.GetPostInput) []post_models.GetPostOutput {
	results := make([]post_models.GetPostOutput, 0, len(input.PostIDs))
	postsMap := fetchPostsCascade(ctx, input.PostIDs)

	// 5. & 6. EMPAQUETAGE ET VÉRIFICATION DES DROITS
	for _, id := range input.PostIDs {
		post, found := postsMap[id]

		if !found {
			results = append(results, post_models.GetPostOutput{PostID: id, Error: "Post introuvable ou supprimé"})
			continue
		}

		// L'auteur a toujours accès à son propre post
		isAuthor := post.UserID == input.UserID

		// ─────────────────────────────────────────────────────────────────
		// HYDRATATION DE L'ÉTAT DE RELATION EN O(1) CASCADE (L1->L2->L3)
		// ─────────────────────────────────────────────────────────────────
		// state: 0 = Rien, 1 = Follower, 2 = Ami, -1 = Banni
		relationState := 0
		if !isAuthor {
			relationState = cache_service.RelationValue(ctx, post.UserID, input.UserID)
		}

		// Règle Z : Bannissement (Si l'auteur a bloqué le caller, ou inversement)
		if relationState == -1 {
			results = append(results, post_models.GetPostOutput{PostID: id, Error: "Post introuvable ou supprimé"})
			continue // Mode furtif : on lui fait croire que le post n'existe pas.
		}

		// ─────────────────────────────────────────────────────────────────
		// Règle A : Soft Delete / Supprimé (Visibility = -1)
		// ─────────────────────────────────────────────────────────────────
		if post.Visibility == -1 {
			results = append(results, post_models.GetPostOutput{PostID: id, Error: "Post introuvable ou supprimé"})
			continue
		}

		// ─────────────────────────────────────────────────────────────────
		// Règle B : Réservé aux abonnés (Visibility = 1)
		// ─────────────────────────────────────────────────────────────────
		if post.Visibility == 1 && !isAuthor {
			// Doit être au moins Abonné (1) ou Ami (2)
			if relationState < 1 {
				results = append(results, post_models.GetPostOutput{
					PostID: id,
					Error:  "🔒 Ce post est privé et strictement réservé aux abonnés de l'auteur",
				})
				continue
			}
		}

		// ─────────────────────────────────────────────────────────────────
		// Règle C : Réservé aux AMIS (Visibility = 2)
		// ─────────────────────────────────────────────────────────────────
		if post.Visibility == 2 && !isAuthor {
			// Doit être strictement Ami (2)
			if relationState != 2 {
				results = append(results, post_models.GetPostOutput{
					PostID: id,
					Error:  "🤝 Ce post est confidentiel et réservé au cercle d'amis de l'auteur",
				})
				continue
			}
		}

		// ─────────────────────────────────────────────────────────────────
		// Règle D : Public (Visibility = 0) ou accès validé
		// ─────────────────────────────────────────────────────────────────

		var mediaURLs []string

		// ⚡ HYDRATATION DES MEDIAS ET SIGNATURE HMAC
		for _, mediaID := range post.MediaIDs {
			// On récupère le fameux storage_path
			mediaPayload, err := media_service.GetMediaCascade(ctx, mediaID)

			// Si le média existe et n'a pas été supprimé par l'auteur
			if err == nil && mediaPayload.Visibility {
				// On génère l'URL avec le vrai storage_path !
				signedURL := media_service.GenerateWatermarkedURL(
					mediaPayload.StoragePath, // "users/12/posts/45/uuid.avif"
					post.UserID,              // L'auteur
					post.ID,                  // Le Post
					input.UserID,             // Le Lecteur
				)
				mediaURLs = append(mediaURLs, signedURL)
			}
		}

		// ⚡ HYDRATATION DES COMMENTAIRES (Via le service dédié optimisé)
		commentInput := comment_models.GetCommentsInput{
			PostID: id,
			UserID: input.UserID,
			Limit:  100, // On s'aligne sur notre Cap L1 ZSET !
			Offset: 0,
		}
		comments, _ := comment_service.GetComments(ctx, commentInput)

		val := post
		results = append(results, post_models.GetPostOutput{
			PostID:   id,
			Data:     &val,
			Media:    mediaURLs, // Le client reçoit les URLs prêtes à l'emploi
			Comments: comments,  // ✅ Injection instantanée de l'arbre des commentaires
		})
	}

	return results
}

// fetchPostsCascade gère la récupération L1 -> L2 -> L3 pour un batch d'IDs.
func fetchPostsCascade(ctx context.Context, ids []int64) map[int64]post_models.PostPayload {
	postsMap := make(map[int64]post_models.PostPayload)
	var missingFromL1 []int64

	// Étape 1 : Object Cache LFU (Redis)
	for _, id := range ids {
		if p, err := object_cache_service.GetPostFromObjectCache(ctx, id); err == nil {
			postsMap[id] = p
		} else {
			missingFromL1 = append(missingFromL1, id)
		}
	}

	if len(missingFromL1) == 0 {
		return postsMap // Tous les posts étaient en RAM, retour instantané
	}

	// Étape 2 : Cold Storage (MongoDB)
	var missingFromL2 []int64
	mongoPosts, errMongo := mongo.MongoLoadPosts(missingFromL1)
	if errMongo == nil {
		for _, p := range mongoPosts {
			postsMap[p.ID] = p
			_ = object_cache_service.SetPostInObjectCache(ctx, p) // Réhydratation L1
		}
	}

	// Identification de ce qu'il reste à trouver
	for _, id := range missingFromL1 {
		if _, exists := postsMap[id]; !exists {
			missingFromL2 = append(missingFromL2, id)
		}
	}

	if len(missingFromL2) == 0 {
		return postsMap
	}

	// Étape 3 : Source of Truth (PostgreSQL) via ta fonction paramétrée
	pgPosts, errPg := postgres.FuncLoadPosts(missingFromL2, len(missingFromL2), 0)
	if errPg == nil {
		for _, p := range pgPosts {
			postsMap[p.ID] = p

			// A. Réhydratation du stockage à froid L2 (MongoDB) pour soulager définitivement Postgres
			_ = mongo.MongoUpsertPost(p)

			// B. Réhydratation du cache haute performance L1 (Redis JSON)
			_ = object_cache_service.SetPostInObjectCache(ctx, p)
		}
	}

	return postsMap
}
