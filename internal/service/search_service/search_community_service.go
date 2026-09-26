package search_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// ############################################################################
// # SERVICE : RECHERCHE DE COMMUNAUTÉS
// ############################################################################

// SearchCommunities orchestre la recherche ultrarapide de communautés publiques
// via le Speed Cache (O(log(N))) et génère les URL signées pour les avatars.
func SearchCommunities(ctx context.Context, callerID int64, input search_models.CommunitySearchInput) (search_models.CommunitySearchOutput, error) {

	// ── ÉTAPE 1 : RÉSOLUTION DE L'INDEX DANS LE SPEED CACHE (L1) ────────────

	liteCommunitiesResults, errRedis := cache_service.SearchCommunitiesByPrefix(ctx, input.Prefix, input.Limit)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Str("prefix", input.Prefix).Msg("Erreur L1 lors de la recherche des communautés")
		return search_models.CommunitySearchOutput{}, nubo_error.NewInternal()
	}

	if len(liteCommunitiesResults) == 0 {
		return search_models.CommunitySearchOutput{
			Communities: make([]conversation_models.CommunityLiteView, 0),
		}, nil
	}

	// ── ÉTAPE 2 : HYDRATATION EN MASSE VIA LE DOMAINE MÉDIA ─────────────────

	hydratedCommunityViews := make([]conversation_models.CommunityLiteView, 0, len(liteCommunitiesResults))

	for _, communityLite := range liteCommunitiesResults {

		var resolvedAvatar media_models.MediaView

		if communityLite.ProfilePictureID > 0 {
			if mediaView, errMedia := media_service.GenerateMediaViewCascade(ctx, communityLite.ProfilePictureID, communityLite.ID, 0, callerID); errMedia == nil {
				resolvedAvatar = mediaView
			}
		}

		hydratedCommunityViews = append(hydratedCommunityViews, conversation_models.CommunityLiteView{
			ID:          communityLite.ID,
			Name:        communityLite.Name,
			Avatar:      resolvedAvatar,
			Description: communityLite.Description,
			MemberCount: communityLite.MemberCount,
		})
	}

	return search_models.CommunitySearchOutput{
		Communities: hydratedCommunityViews,
	}, nil
}
