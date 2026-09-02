package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// GetAddableUsers délègue la résolution paginée au service de cache hybride puis hydrate les avatars
func GetAddableUsers(ctx context.Context, callerID int64, input relation_models.GetAddableInput) (relation_models.GetAddableOutput, error) {
	// 1. Appel du Cache Service (Pur DDD : Aucune URL générée ici)
	liteUsers, err := cache_service.GetAddableUsersFromSpeedCache(ctx, callerID, input.Limit, input.Offset, input.Force)
	if err != nil {
		return relation_models.GetAddableOutput{}, err
	}

	// 2. Hydratation via le Domaine Média
	views := make([]auth_models.UserLiteView, 0, len(liteUsers))

	for _, u := range liteUsers {
		var avatar media_models.MediaView // Zéro valeur {MediaID: 0, URL: ""}

		if u.ProfilePictureID > 0 {
			// authorID = u.ID (c'est l'auteur de sa propre photo), targetID = 0 (pas de post)
			if view, errMedia := media_service.GenerateMediaViewCascade(ctx, u.ProfilePictureID, u.ID, 0, callerID); errMedia == nil {
				avatar = view
			}
		}

		// Composition par valeur stricte
		views = append(views, auth_models.UserLiteView{
			User:   u,
			Avatar: avatar,
		})
	}

	return relation_models.GetAddableOutput{Users: views}, nil
}
