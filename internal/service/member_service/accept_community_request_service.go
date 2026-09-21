package member_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// AcceptCommunityRequest valide l'adhésion d'un membre en attente (Role = -3).
func AcceptCommunityRequest(ctx context.Context, callerID int64, input member_models.AcceptCommunityRequestInput) (member_models.AcceptCommunityRequestOutput, error) {
	// 1. SÉCURITÉ : L'appelant doit être Admin (1) ou Owner (2)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return member_models.AcceptCommunityRequestOutput{}, nubo_error.NewForbidden("ACCESS_DENIED", "Conversation introuvable ou accès refusé.", err)
	}
	if callerMem.Role < 1 {
		return member_models.AcceptCommunityRequestOutput{}, nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Vous devez être administrateur pour accepter une candidature.", nil)
	}

	// 2. RÉCUPÉRATION DU MEMBRE CIBLE (Cascade L1 -> L2 -> L3 manuelle car LeftMember bloque les rôles < 0)
	var targetMem member_models.MemberPayload
	targetMem, err = object_cache_service.GetMemberFromObjectCache(ctx, input.ConversationID, input.TargetUserID)
	if err != nil || targetMem.ID == 0 {
		targetMem, err = mongo.MongoGetMember(input.ConversationID, input.TargetUserID)
		if err != nil || targetMem.ID == 0 {
			targetMem, err = postgres.FuncGetMember(ctx, input.ConversationID, input.TargetUserID)
			if err != nil || targetMem.ID == 0 {
				return member_models.AcceptCommunityRequestOutput{}, nubo_error.NewNotFound("USER_NOT_FOUND", "Candidature introuvable.", err)
			}
		}
	}

	// 3. RÈGLE MÉTIER : Vérifier l'état d'attente
	if targetMem.Role != -3 {
		return member_models.AcceptCommunityRequestOutput{}, nubo_error.NewBadRequest("INVALID_STATE", "Cet utilisateur n'est pas en attente d'approbation.", nil)
	}

	// 4. APPLICATION DE L'ACCEPTATION
	now := time.Now().UTC()
	targetMem.Role = 0 // Devient membre officiel
	targetMem.Settings = member_models.DefaultMemberSettings(3)
	targetMem.JoinedAt = domain.TimeToMillis(now)
	targetMem.UpdatedAt = domain.TimeToMillis(now)

	// 5. MISE À JOUR SYNCHRONE DES CACHES (L1)
	_ = object_cache_service.SetMemberInObjectCache(ctx, targetMem)

	// Mise à jour de l'index Speed Cache
	_ = cache_service.UpdateMemberSpeedCache(ctx, lite_models.MemberLiteRequest{
		ConversationID:  targetMem.ConversationID,
		UserID:          targetMem.UserID,
		Role:            targetMem.Role,
		Settings:        service.ToMemberSettingsLite(targetMem.Settings),
		UnreadCount:     targetMem.UnreadCount,
		FrozenMessageID: targetMem.FrozenMessageID,
		JoinedAt:        targetMem.JoinedAt,
	})

	// 6. ENVOI AUX WORKERS (Write-Behind)
	err = redis.EnqueueDB(ctx, targetMem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, targetMem, redis.TargetAll)
	if err != nil {
		return member_models.AcceptCommunityRequestOutput{}, err
	}

	// 7. MESSAGE SYSTÈME ET NOTIFICATION (Asynchrone)
	go func() {
		bgCtx := context.Background()

		// A. On récupère le pseudo pour le message système
		if targetLite, errLite := cache_service.GetUserLite(bgCtx, targetMem.UserID); errLite == nil {
			sysContent := fmt.Sprintf("%s a rejoint le groupe", targetLite.Username)
			msgInput := message_models.CreateMessageInput{
				MessageType: 8,
				Content:     sysContent,
			}

			// Le créateur du message système est souvent l'Admin qui accepte, ou le système lui-même.
			// Ici on utilise le callerID comme instigateur technique.
			_, _ = message_service.CreateMessage(bgCtx, callerID, input.ConversationID, msgInput, true)

			// B. Hydratation du DTO WebSocket pour la diffusion temps réel
			conv, _ := object_cache_service.GetConversationFromObjectCache(bgCtx, input.ConversationID)

			memView := member_models.MemberView{
				MemberPayload: targetMem,
				Username:      targetLite.Username,
				IsOnline:      cache_service.IsUserOnline(bgCtx, targetMem.UserID), // NOUVEAU
			}

			if conv.Type == 2 || conv.Type == 3 {
				memView.AvatarCommunityID = targetLite.ProfilePictureID // Mode Twitch
			} else {
				if targetLite.ProfilePictureID > 0 {
					// Signature HMAC
					if avatarView, errMedia := media_service.GenerateMediaViewCascade(bgCtx, targetLite.ProfilePictureID, targetMem.UserID, 0, callerID); errMedia == nil {
						memView.Avatar = avatarView
					}
				}
			}

			// Diffusion de l'événement global
			_ = realtime_service.BroadcastToConversation(bgCtx, input.ConversationID, "member.joined", memView)
		}
	}()

	output := member_models.AcceptCommunityRequestOutput{}

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé TOUT À LA FIN de la fonction. Cela écrase tout timestamp qui aurait
	// pu être généré précédemment (par ex. à l'intérieur de AddMembersToConversation)
	// et garantit que le client reçoit la date de la fin absolue de la transaction.
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	output.InboxUpdateAt = domain.TimeToMillis(time.UnixMilli(timestampMs))

	return output, nil
}
