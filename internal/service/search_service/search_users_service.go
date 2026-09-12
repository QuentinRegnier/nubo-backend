package search_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// SearchUsers orchestre la recherche ultrarapide via le Speed Cache et hydrate les avatars.
func SearchUsers(ctx context.Context, callerID int64, input search_models.UserSearchInput) (search_models.UserSearchOutput, error) {
	// 1. Appel du Cache Service (Pur DDD : O(log(N)) en RAM, aucun fallback BDD pour garantir < 5ms)
	liteUsers, err := cache_service.SearchUserByPrefix(ctx, input.Prefix, input.Limit)
	if err != nil {
		return search_models.UserSearchOutput{}, err
	}

	if len(liteUsers) == 0 {
		return search_models.UserSearchOutput{Users: make([]auth_models.UserLiteView, 0)}, nil // Tableau vide propre
	}

	// 2. Hydratation via le Domaine Média
	views := make([]auth_models.UserLiteView, 0, len(liteUsers))
	for _, u := range liteUsers {
		var avatar media_models.MediaView // Zéro-valeur {MediaID: 0, URL: ""}

		if u.ProfilePictureID > 0 {
			// authorID = u.ID (c'est l'auteur de sa propre photo), targetID = 0 (pas de post)
			if view, errMedia := media_service.GenerateMediaViewCascade(ctx, u.ProfilePictureID, u.ID, 0, callerID); errMedia == nil {
				avatar = view
			}
		}

		// Composition par valeur stricte
		views = append(views, auth_models.UserLiteView{
			User:     u,
			Avatar:   avatar,
			IsOnline: cache_service.IsUserOnline(ctx, u.ID),
		})
	}

	return search_models.UserSearchOutput{Users: views}, nil
}
