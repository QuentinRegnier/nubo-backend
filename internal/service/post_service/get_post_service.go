package post_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/comment_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION ET FORMATAGE D'UN LOT DE POSTS
// ############################################################################

// GetPosts orchestre la récupération d'une liste de posts, applique le filtrage de
// visibilité et hydrate massivement les relations (Commentaires, Médias, Auteur).
func GetPosts(ctx context.Context, input post_models.GetPostInput) []post_models.GetPostOutput {

	hydratedPostsResults := make([]post_models.GetPostOutput, 0, len(input.PostIDs))

	// Utilisation de l'assistant de cascade (Helpers) pour le chargement brut des payloads
	resolvedPostsMap := fetchPostsCascade(ctx, input.PostIDs)

	for _, requestedPostID := range input.PostIDs {
		postPayload, isFound := resolvedPostsMap[requestedPostID]

		if !isFound {
			hydratedPostsResults = append(hydratedPostsResults, post_models.GetPostOutput{
				PostID: requestedPostID,
				Error:  "Cette publication est introuvable ou a été supprimée.",
			})
			continue
		}

		// L'auteur a toujours accès inconditionnel à son propre post
		isCallerAuthor := postPayload.UserID == input.UserID

		// ── ÉTAPE 1 : MATRICE DE VISIBILITÉ ET RELATIONNELLE ────────────────

		// State: 0 = Rien, 1 = Abonné, 2 = Ami, -1 = Banni
		relationState := 0
		if !isCallerAuthor {
			relationState = cache_service.RelationValue(ctx, postPayload.UserID, input.UserID)
		}

		// Règle Z : Bannissement Croisé
		if relationState == variables.RelationStateBlocked {
			hydratedPostsResults = append(hydratedPostsResults, post_models.GetPostOutput{
				PostID: requestedPostID,
				Error:  "Cette publication est introuvable ou a été supprimée.", // Mode furtif
			})
			continue
		}

		// Règle A : Soft Delete
		if postPayload.Visibility == variables.PostVisibilityDeleted {
			hydratedPostsResults = append(hydratedPostsResults, post_models.GetPostOutput{
				PostID: requestedPostID,
				Error:  "Cette publication est introuvable ou a été supprimée.",
			})
			continue
		}

		// Règle B : Réservé aux Abonnés
		if postPayload.Visibility == variables.PostVisibilitySubcriber && !isCallerAuthor {
			if relationState < variables.RelationStateFollow {
				hydratedPostsResults = append(hydratedPostsResults, post_models.GetPostOutput{
					PostID: requestedPostID,
					Error:  "🔒 Ce post est strictement réservé aux abonnés de l'auteur.",
				})
				continue
			}
		}

		// Règle C : Réservé aux Amis
		if postPayload.Visibility == variables.PostVisibilityFriend && !isCallerAuthor {
			if relationState != variables.RelationStateFriend {
				hydratedPostsResults = append(hydratedPostsResults, post_models.GetPostOutput{
					PostID: requestedPostID,
					Error:  "🤝 Ce post est confidentiel et réservé au cercle d'amis de l'auteur.",
				})
				continue
			}
		}

		// ── ÉTAPE 2 : HYDRATATION DU DTO (AUTEUR, MÉDIAS, COMMENTAIRES) ─────

		var resolvedAuthorUsername string
		var resolvedAuthorAvatar media_models.MediaView

		if authorUserLite, errLite := cache_service.GetUserLite(ctx, postPayload.UserID); errLite == nil {
			resolvedAuthorUsername = authorUserLite.Username

			if authorUserLite.ProfilePictureID > 0 {
				if avatarView, errMedia := media_service.GenerateMediaViewCascade(ctx, authorUserLite.ProfilePictureID, postPayload.UserID, postPayload.ID, input.UserID); errMedia == nil {
					resolvedAuthorAvatar = avatarView
				}
			}
		}

		// Signature HMAC des médias via le domaine dédié
		signedMediaViews := media_service.FormatMediaViewsCascade(ctx, postPayload.MediaIDs, postPayload.UserID, postPayload.ID, input.UserID)

		// Chargement direct des Top Commentaires
		commentRequestInput := comment_models.GetCommentsInput{
			PostID: requestedPostID,
			UserID: input.UserID,
			Limit:  variables.MaxZsetPostComment, // Aligné sur le Cap L1 ZSET
			Offset: 0,
		}
		topCommentsList, _ := comment_service.GetComments(ctx, commentRequestInput)

		// ── ÉTAPE 3 : ASSEMBLAGE FINAL DE LA VUE ────────────────────────────

		hydratedPostsResults = append(hydratedPostsResults, post_models.GetPostOutput{
			PostID:         requestedPostID,
			Data:           postPayload,
			AuthorUsername: resolvedAuthorUsername,
			AuthorAvatar:   resolvedAuthorAvatar,
			Media:          signedMediaViews,
			Comments:       topCommentsList,
		})
	}

	return hydratedPostsResults
}
