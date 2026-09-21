package member_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// GetBannedMembers retourne la liste paginée des utilisateurs bannis
func GetBannedMembers(ctx context.Context, callerID int64, input member_models.GetBannedMembersInput) (member_models.GetBannedMembersOutput, error) {
	// 1. SÉCURITÉ : Vérifier que l'appelant est Admin ou Propriétaire (Role >= 1)
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil || mem.Role < 1 {
		return member_models.GetBannedMembersOutput{}, nubo_error.NewForbidden("ACCESS_DENIED", "Seuls les administrateurs peuvent voir la liste des bannis.", nil)
	}

	// 2. RÉCUPÉRATION L3 : On interroge directement Postgres car les bannis ne sont pas gardés dans les Sets de distribution L1
	bannedMembers, err := postgres.FuncLoadMembersByRolePaginated(ctx, input.ConversationID, -2, input.Limit, input.Offset)
	if err != nil {
		return member_models.GetBannedMembersOutput{}, nubo_error.NewInternal(err)
	}

	var bannedUsers []member_models.BannedUserView

	// 3. HYDRATATION : Construction de la vue pour le frontend
	for _, bMem := range bannedMembers {
		if userLite, errLite := cache_service.GetUserLite(ctx, bMem.UserID); errLite == nil {
			var avatarView media_models.MediaView
			if userLite.ProfilePictureID > 0 {
				if view, errMedia := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, bMem.UserID, 0, callerID); errMedia == nil {
					avatarView = view
				}
			}

			bannedUsers = append(bannedUsers, member_models.BannedUserView{
				UserLiteView: auth_models.UserLiteView{
					User:     userLite,
					Avatar:   avatarView,
					IsOnline: cache_service.IsUserOnline(ctx, bMem.UserID),
				},
				BanDate: bMem.UpdatedAt, // L'UpdateAt correspond au moment exact du ban
			})
		}
	}

	if bannedUsers == nil {
		bannedUsers = make([]member_models.BannedUserView, 0)
	}

	return member_models.GetBannedMembersOutput{BannedUsers: bannedUsers}, nil
}
