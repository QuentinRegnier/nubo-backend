package conversation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// GetMutedMembers récupère la liste des utilisateurs mutés (Accessible aux Admins/Owners).
func GetMutedMembers(ctx context.Context, callerID int64, input conversation_models.GetMutedMembersInput) (conversation_models.GetMutedMembersOutput, error) {
	// 1. SÉCURITÉ : Vérifier que le Caller est Admin (1) ou Propriétaire (2)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return conversation_models.GetMutedMembersOutput{}, err
	}
	if callerMem.Role < 1 {
		return conversation_models.GetMutedMembersOutput{}, nubo_error.NewForbidden("NOT_ADMIN", "Seuls les administrateurs ont accès à cette liste.", nil)
	}

	// 2. RÉCUPÉRATION BDD L3 (Source of Truth)
	records, errPg := postgres.FuncLoadMutedMembersPaginated(ctx, input.ConversationID, input.Limit, input.Offset)
	if errPg != nil {
		return conversation_models.GetMutedMembersOutput{}, nubo_error.NewInternal(errPg)
	}

	// 3. HYDRATATION EN MASSE VIA SPEED CACHE (L1)
	var mutedViews []conversation_models.MutedUserView
	for _, rec := range records {
		if userLite, errLite := cache_service.GetUserLite(ctx, rec.UserID); errLite == nil {
			var avatarView media_models.MediaView
			if userLite.ProfilePictureID > 0 {
				if view, errMedia := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, rec.UserID, 0, callerID); errMedia == nil {
					avatarView = view
				}
			}

			mutedViews = append(mutedViews, conversation_models.MutedUserView{
				UserLiteView: auth_models.UserLiteView{
					User:     userLite,
					Avatar:   avatarView,
					IsOnline: cache_service.IsUserOnline(ctx, rec.UserID),
				},
				RestrictedUntil: rec.RestrictedUntil,
			})
		}
	}

	if mutedViews == nil {
		mutedViews = make([]conversation_models.MutedUserView, 0)
	}

	return conversation_models.GetMutedMembersOutput{MutedUsers: mutedViews}, nil
}
