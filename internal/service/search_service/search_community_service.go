package search_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// SearchCommunities orchestre la recherche ultrarapide via le Speed Cache et hydrate les avatars.
func SearchCommunities(ctx context.Context, callerID int64, input search_models.CommunitySearchInput) (search_models.CommunitySearchOutput, error) {
	// 1. Appel du Cache Service (Pur DDD : O(log(N)) en RAM)
	liteCommunities, err := cache_service.SearchCommunitiesByPrefix(ctx, input.Prefix, input.Limit)
	if err != nil {
		return search_models.CommunitySearchOutput{}, err
	}

	if len(liteCommunities) == 0 {
		return search_models.CommunitySearchOutput{Communities: make([]conversation_models.CommunityLiteView, 0)}, nil
	}

	// 2. Hydratation via le Domaine Média
	views := make([]conversation_models.CommunityLiteView, 0, len(liteCommunities))
	for _, c := range liteCommunities {
		var avatar media_models.MediaView // Zéro-valeur

		if c.ProfilePictureID > 0 {
			if view, errMedia := media_service.GenerateMediaViewCascade(ctx, c.ProfilePictureID, c.ID, 0, callerID); errMedia == nil {
				avatar = view
			}
		}

		views = append(views, conversation_models.CommunityLiteView{
			ID:          c.ID,
			Name:        c.Name,
			Avatar:      avatar,
			Description: c.Description,
			MemberCount: c.MemberCount,
		})
	}

	return search_models.CommunitySearchOutput{Communities: views}, nil
}
