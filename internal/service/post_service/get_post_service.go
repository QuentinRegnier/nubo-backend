package post_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models" // ✅ NOUVEL IMPORT NÉCESSAIRE
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
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

		// ✅ NOUVEAU : HYDRATATION DE L'AUTEUR (Pseudo + Avatar en O(1))
		var authorUsername string
		var authorAvatar media_models.MediaView

		if authorLite, errLite := cache_service.GetUserLite(ctx, post.UserID); errLite == nil {
			authorUsername = authorLite.Username
			if authorLite.ProfilePictureID > 0 {
				// authorID = post.UserID, targetID = post.ID, readerID = input.UserID
				if view, errMedia := media_service.GenerateMediaViewCascade(ctx, authorLite.ProfilePictureID, post.UserID, post.ID, input.UserID); errMedia == nil {
					authorAvatar = view
				}
			}
		}

		// HYDRATATION DES MEDIAS ET SIGNATURE HMAC VIA LE DOMAINE DÉDIÉ
		mediaURLs := media_service.FormatMediaViewsCascade(ctx, post.MediaIDs, post.UserID, post.ID, input.UserID)

		// HYDRATATION DES COMMENTAIRES (Via le service dédié optimisé)
		commentInput := comment_models.GetCommentsInput{
			PostID: id,
			UserID: input.UserID,
			Limit:  100, // On s'aligne sur notre Cap L1 ZSET !
			Offset: 0,
		}
		comments, _ := comment_service.GetComments(ctx, commentInput)

		results = append(results, post_models.GetPostOutput{
			PostID:         id,
			Data:           post,           // Affectation directe
			AuthorUsername: authorUsername, // ✅ Rempli
			AuthorAvatar:   authorAvatar,   // ✅ Rempli
			Media:          mediaURLs,
			Comments:       comments,
		})
	}

	return results
}
