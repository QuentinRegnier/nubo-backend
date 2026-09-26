package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # UTILITAIRES : RÉCUPÉRATION ET HYDRATATION DES RELATIONS
// ############################################################################

// fetchRelationsHydrated est le moteur central du domaine relationnel.
// Il gère la pagination via le Speed Cache L1, le fallback vers PostgreSQL,
// l'application des règles de confidentialité et l'hydratation massive (O(1)) des profils.
func fetchRelationsHydrated(ctx context.Context, callerID int64, primaryTargetID int64, targetState int, searchDirection string, fetchLimit int, fetchOffset int) ([]relation_models.RelationUserView, error) {

	// ── ÉTAPE 1 : CONTRÔLE DE CONFIDENTIALITÉ (ZERO-TRUST) ──────────────────

	primaryUserLite, errCache := cache_service.GetUserLite(ctx, primaryTargetID)
	if errCache != nil {
		return []relation_models.RelationUserView{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Nous ne trouvons pas ce profil.", nil)
	}

	// L'utilisateur peut toujours voir ses propres relations
	if callerID != primaryTargetID {
		relationStateWithTarget := cache_service.RelationValue(ctx, primaryTargetID, callerID)

		canViewConnections := false

		switch primaryUserLite.HideConnections {
		case variables.ConnectionsVisibilityPublic:
			canViewConnections = true
		case variables.ConnectionsVisibilityFollowers:
			canViewConnections = relationStateWithTarget >= variables.RelationStateFollow
		case variables.ConnectionsVisibilityFriends:
			canViewConnections = relationStateWithTarget == variables.RelationStateFriend
		case variables.ConnectionsVisibilityPrivate:
			canViewConnections = false
		}

		// Règle de blocage absolue
		if relationStateWithTarget == variables.RelationStateBlocked {
			canViewConnections = false
		}

		if !canViewConnections {
			return []relation_models.RelationUserView{}, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Vous n'avez pas la permission de consulter cette liste.", nil)
		}
	}

	// ── ÉTAPE 2 : RÉSOLUTION DE L'INDEX (L1 -> L3 BYPASS MONGO) ─────────────

	// TENTATIVE L1 (ZSET RAM Parfaitement Temporel & Continu)
	matchedUserIDs, errCacheIndex := cache_service.GetRelationsByDirectionPaginatedFromCache(ctx, primaryTargetID, targetState, searchDirection, fetchLimit, fetchOffset)

	// FALLBACK L3 (PostgreSQL - Bypass total de Mongo L2 en raison du "Hole problem")
	if errCacheIndex != nil || len(matchedUserIDs) == 0 {
		matchedUserIDsFromPg, errPg := postgres.FuncLoadRelationsByDirectionPaginated(ctx, primaryTargetID, targetState, searchDirection, fetchLimit, fetchOffset)
		if errPg != nil {
			logger.Log.Error().Err(errPg).Int64("user_id", primaryTargetID).Msg("Échec L3 lors de la récupération des relations")
			return nil, nubo_error.NewInternal()
		}
		matchedUserIDs = matchedUserIDsFromPg
	}

	// ── ÉTAPE 3 : HYDRATATION EN MASSE VIA SPEED CACHE (L1) ─────────────────

	var hydratedUserViews []relation_models.RelationUserView

	for _, matchedID := range matchedUserIDs {
		if matchedUserLite, errLite := cache_service.GetUserLite(ctx, matchedID); errLite == nil {

			var userAvatarView media_models.MediaView
			if matchedUserLite.ProfilePictureID > 0 {
				if generatedView, errMedia := media_service.GenerateMediaViewCascade(ctx, matchedUserLite.ProfilePictureID, matchedID, 0, callerID); errMedia == nil {
					userAvatarView = generatedView
				}
			}

			// Récupération de la relation existante entre l'appelant et l'utilisateur affiché dans la liste
			viewerRelationState := cache_service.RelationValue(ctx, matchedID, callerID)

			hydratedUserViews = append(hydratedUserViews, relation_models.RelationUserView{
				UserLiteView: auth_models.UserLiteView{
					User:     matchedUserLite,
					Avatar:   userAvatarView,
					IsOnline: cache_service.IsUserOnline(ctx, matchedID),
				},
				ViewerRelationState: viewerRelationState,
			})
		}
	}

	// Prévention stricte du `null` en JSON
	if hydratedUserViews == nil {
		hydratedUserViews = make([]relation_models.RelationUserView, 0)
	}

	return hydratedUserViews, nil
}
