package conversation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UnbanMembers débannit un lot d'utilisateurs
func UnbanMembers(ctx context.Context, callerID int64, input conversation_models.UnbanMembersInput) (conversation_models.UnbanMembersOutput, error) {
	// 1. SÉCURITÉ : Vérifier que l'appelant est Admin ou Propriétaire
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil || mem.Role < 1 {
		return conversation_models.UnbanMembersOutput{}, nubo_error.NewForbidden("ACCESS_DENIED", "Seuls les administrateurs peuvent débannir des utilisateurs.", nil)
	}

	now := service.NowMillis()

	for _, targetID := range input.TargetUserIDs {
		// Récupérer le membre ciblé
		targetMem, errMem := security_service.LeftMember(ctx, input.ConversationID, targetID)
		if errMem != nil {
			continue // S'il n'existe pas, on passe au suivant
		}

		// Si l'utilisateur est bien banni
		if targetMem.Role == -2 {
			// On modifie le rôle à -1 (Quit). Ainsi, l'utilisateur n'est plus banni et pourra rejoindre librement la communauté à l'avenir.
			targetMem.Role = -1
			targetMem.UpdatedAt = now
			targetMem.FrozenMessageID = 0 // On enlève le plafond

			// Mise à jour RAM (L1)
			_ = cache_service.UpdateMemberSpeedCache(ctx, lite_models.MemberLiteRequest{
				ConversationID:  targetMem.ConversationID,
				UserID:          targetMem.UserID,
				Role:            targetMem.Role,
				Settings:        service.ToMemberSettingsLite(targetMem.Settings),
				UnreadCount:     targetMem.UnreadCount,
				FrozenMessageID: targetMem.FrozenMessageID,
				JoinedAt:        targetMem.JoinedAt,
			})

			// Write-Behind pour L2 / L3
			_ = redis.EnqueueDB(ctx, targetMem.ID, targetMem.ConversationID, redis.EntityMembers, redis.ActionUpdate, targetMem, redis.TargetAll)
		}
	}

	// 4. Dirty Flag de la boite aux lettres de l'admin
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	return conversation_models.UnbanMembersOutput{InboxUpdateAt: timestampMs}, nil
}
