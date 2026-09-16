package conversation_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// PromoteMember promeut un membre au rang d'administrateur (Rôle = 1)
func PromoteMember(ctx context.Context, callerID int64, input conversation_models.PromoteMemberInput) (conversation_models.PromoteMemberOutput, error) {
	// 1. SÉCURITÉ : Vérification des droits du Caller (L1 -> L2 -> L3)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return conversation_models.PromoteMemberOutput{}, nubo_error.NewForbidden("ACCESS_DENIED", "Conversation introuvable ou accès refusé.", err)
	}
	if callerMem.Role != 2 {
		return conversation_models.PromoteMemberOutput{}, nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Seul le propriétaire peut promouvoir un membre.", nil)
	}

	// 2. RÉCUPÉRATION DU MEMBRE CIBLE
	targetMem, err := security_service.LeftMember(ctx, input.ConversationID, input.TargetUserID)
	if err != nil || targetMem.Role < 0 {
		return conversation_models.PromoteMemberOutput{}, nubo_error.NewBadRequest("USER_NOT_MEMBER", "L'utilisateur ciblé n'est pas membre de ce groupe.", err)
	}

	// 3. IDEMPOTENCE : Si la cible est déjà Admin (1) ou Propriétaire (2), on s'arrête là silencieusement
	if targetMem.Role >= 1 {
		return conversation_models.PromoteMemberOutput{}, nil
	}

	// 4. APPLICATION DE LA MODIFICATION
	targetMem.Role = 1
	targetMem.UpdatedAt = service.NowMillis()

	callerLite, _ := cache_service.GetUserLite(ctx, callerID)
	targetLite, _ := cache_service.GetUserLite(ctx, input.TargetUserID)
	sysContent := fmt.Sprintf("%s a promu %s", callerLite.Username, targetLite.Username)
	msgInput := message_models.CreateMessageInput{
		MessageType: 8,
		Content:     sysContent,
	}
	_, _ = message_service.CreateMessage(ctx, callerID, input.ConversationID, msgInput, true)

	// 5. MISE À JOUR DE L'Object Cache (Payload Complet LFU)
	_ = object_cache_service.SetMemberInObjectCache(ctx, targetMem)

	// === NOUVEAU : MISE À JOUR SYNCHRONE DU SPEED CACHE ===
	_ = cache_service.UpdateMemberSpeedCache(ctx, lite_models.MemberLiteRequest{
		ConversationID:  targetMem.ConversationID,
		UserID:          targetMem.UserID,
		Role:            targetMem.Role,
		Settings:        service.ToMemberSettingsLite(targetMem.Settings),
		UnreadCount:     targetMem.UnreadCount,
		FrozenMessageID: targetMem.FrozenMessageID,
		JoinedAt:        targetMem.JoinedAt,
	})

	// 7. ENVOI AUX WORKERS (Write-Behind Asynchrone)
	// PartitionKey = ConversationID pour exécuter les requêtes séquentiellement pour ce groupe
	err = redis.EnqueueDB(ctx, targetMem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, targetMem, redis.TargetAll)

	// 6. ENVOI NOTIFICATION (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), input.ConversationID, "member.promoted", targetMem)
			if err != nil {
				logger.Log.Error().Err(err).Msg("Erreur lors de l'envoi de la notification de promotion")
			}
		}()
	}

	output := conversation_models.PromoteMemberOutput{}

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé TOUT À LA FIN de la fonction. Cela écrase tout timestamp qui aurait
	// pu être généré précédemment (par ex. à l'intérieur de AddMembersToConversation)
	// et garantit que le client reçoit la date de la fin absolue de la transaction.
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	output.InboxUpdateAt = domain.TimeToMillis(time.UnixMilli(timestampMs))

	return output, err
}
