package search_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// ############################################################################
// # SERVICE : RECHERCHE D'UTILISATEURS (COMPTES)
// ############################################################################

// SearchUsers orchestre la recherche ultrarapide via le Speed Cache Redis (ZSET Lex)
// et déclenche l'hydratation des avatars à la volée.
func SearchUsers(ctx context.Context, callerID int64, input search_models.UserSearchInput) (search_models.UserSearchOutput, error) {

	// ── ÉTAPE 1 : RECHERCHE L1 EN RAM (O(log N)) ────────────────────────────

	// Aucun fallback BDD n'est prévu ici pour garantir un temps de réponse < 5ms
	// indispensable pour l'autocomplétion pendant la frappe utilisateur.
	liteUsersResults, errRedis := cache_service.SearchUserByPrefix(ctx, input.Prefix, input.Limit)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Str("prefix", input.Prefix).Msg("Erreur L1 lors de la recherche des utilisateurs par préfixe")
		return search_models.UserSearchOutput{}, nubo_error.NewInternal()
	}

	// Coupe-circuit sécurisé (Protection contre le retour null)
	if len(liteUsersResults) == 0 {
		return search_models.UserSearchOutput{
			Users: make([]auth_models.UserLiteView, 0),
		}, nil
	}

	// ── ÉTAPE 2 : HYDRATATION VIA LE DOMAINE MÉDIA ──────────────────────────

	hydratedUserViews := make([]auth_models.UserLiteView, 0, len(liteUsersResults))

	for _, userLite := range liteUsersResults {

		var resolvedAvatar media_models.MediaView

		if userLite.ProfilePictureID > 0 {
			// contextID = 0 car l'avatar n'est pas lié contextuellement à un Post/Conversation
			if mediaView, errMedia := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, userLite.ID, 0, callerID); errMedia == nil {
				resolvedAvatar = mediaView
			}
		}

		hydratedUserViews = append(hydratedUserViews, auth_models.UserLiteView{
			User:     userLite,
			Avatar:   resolvedAvatar,
			IsOnline: cache_service.IsUserOnline(ctx, userLite.ID),
		})
	}

	return search_models.UserSearchOutput{
		Users: hydratedUserViews,
	}, nil
}
