package relation_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// FetchRelationsHydrated Bypass MongoDB (Hole problem) : Speed Cache L1 (ZSET) -> PostgreSQL L3
func FetchRelationsHydrated(ctx context.Context, callerID int64, primaryID int64, state int, direction string, limit int, offset int) ([]relation_models.RelationUserView, error) {
	primaryLite, err := cache_service.GetUserLite(ctx, primaryID)
	if err != nil {
		return []relation_models.RelationUserView{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Nous ne trouvons pas ce profil", err)
	}
	relationState := cache_service.RelationValue(ctx, primaryID, callerID)

	canView := false
	switch primaryLite.HideConnections {
	case 0: // Tout le monde peut l'ajouter
		canView = true
	case 1: // Seuls ses amis peuvent l'ajouter
		canView = (relationState == 1)
	case 2: // Personne ne peut l'ajouter (invitation obligatoire)
		canView = (relationState == 2)
	case 3:
		canView = false
	}

	if canView {
		return []relation_models.RelationUserView{}, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Vous n'avez pas la permission de lire cette liste", errors.New("à changer"))
	}

	// 1. TENTATIVE L1 (ZSET RAM Parfaitement Temporel & Continu)
	matchedIDs, err := cache_service.GetRelationsByDirectionPaginatedFromCache(ctx, primaryID, state, direction, limit, offset)

	// 2. FALLBACK L3 (PostgreSQL - Bypass total de Mongo L2)
	if err != nil || len(matchedIDs) == 0 {
		matchedIDsPg, errPg := postgres.FuncLoadRelationsByDirectionPaginated(ctx, primaryID, state, direction, limit, offset)
		if errPg != nil {
			return nil, nubo_error.NewInternal(errPg)
		}
		matchedIDs = matchedIDsPg
	}

	var usersView []relation_models.RelationUserView

	// 3. HYDRATATION EN MASSE VIA SPEED CACHE (L1)
	for _, mID := range matchedIDs {
		if userLite, errLite := cache_service.GetUserLite(ctx, mID); errLite == nil {
			var avatarView media_models.MediaView
			if userLite.ProfilePictureID > 0 {
				if view, errMedia := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, mID, 0, callerID); errMedia == nil {
					avatarView = view
				}
			}

			// ✅ NOUVEAU : Récupération de la relation entre l'appelant et l'utilisateur affiché
			viewerState := cache_service.RelationValue(ctx, mID, callerID)

			usersView = append(usersView, relation_models.RelationUserView{
				UserLiteView: auth_models.UserLiteView{
					User:     userLite,
					Avatar:   avatarView,
					IsOnline: cache_service.IsUserOnline(ctx, mID),
				},
				ViewerRelationState: viewerState,
			})
		}
	}

	if usersView == nil {
		usersView = make([]relation_models.RelationUserView, 0)
	}

	return usersView, nil
}
