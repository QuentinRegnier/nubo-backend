package member_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// RefuseCommunityRequest rejette une candidature en attente et passe le membre au rôle -4 (Rejeté).
func RefuseCommunityRequest(ctx context.Context, callerID int64, input member_models.RefuseCommunityRequestInput) (member_models.RefuseCommunityRequestOutput, error) {
	// 1. SÉCURITÉ : L'appelant doit être Admin (1) ou Owner (2)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return member_models.RefuseCommunityRequestOutput{}, nubo_error.NewForbidden("ACCESS_DENIED", "Conversation introuvable ou accès refusé.", err)
	}
	if callerMem.Role < 1 {
		return member_models.RefuseCommunityRequestOutput{}, nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Vous devez être administrateur pour refuser une candidature.", nil)
	}

	// 2. RÉCUPÉRATION DU MEMBRE CIBLE (Cascade L1 -> L2 -> L3)
	var targetMem member_models.MemberPayload
	targetMem, err = object_cache_service.GetMemberFromObjectCache(ctx, input.ConversationID, input.TargetUserID)

	if err != nil || targetMem.ID == 0 {
		targetMem, err = mongo.MongoGetMember(input.ConversationID, input.TargetUserID)
		if err != nil || targetMem.ID == 0 {
			targetMem, err = postgres.FuncGetMember(ctx, input.ConversationID, input.TargetUserID)
			if err != nil || targetMem.ID == 0 {
				return member_models.RefuseCommunityRequestOutput{}, nubo_error.NewNotFound("USER_NOT_FOUND", "Candidature introuvable.", err)
			}

			// ⬆️ PROMOTION L3 -> L2 (Asynchrone via Worker Mongo)
			go func(m member_models.MemberPayload) {
				bgCtx := context.Background()
				_ = redis.EnqueueDB(bgCtx, m.ID, m.ConversationID, redis.EntityMembers, redis.ActionUpdate, m, redis.TargetMongo)
			}(targetMem)
		}

		// ⬆️ PROMOTION L3/L2 -> L1 (Immédiate en RAM)
		_ = object_cache_service.SetMemberInObjectCache(ctx, targetMem)
	}

	// 3. RÈGLE MÉTIER : Vérifier l'état d'attente
	if targetMem.Role != -3 {
		return member_models.RefuseCommunityRequestOutput{}, nubo_error.NewBadRequest("INVALID_STATE", "Cet utilisateur n'est pas en attente d'approbation.", nil)
	}

	// 4. APPLICATION DU REJET (Role = -4)
	targetMem.Role = -4
	targetMem.UpdatedAt = domain.NowMillis()

	// 5. MISE À JOUR SYNCHRONE DES CACHES (L1)
	_ = object_cache_service.SetMemberInObjectCache(ctx, targetMem)

	// Retrait de la communauté de l'inbox de l'utilisateur (Pour ne pas polluer sa liste)
	_ = cache_service.RemoveMemberFromSpeedCache(ctx, input.ConversationID, input.TargetUserID)

	// Mais on maintient son empreinte Lite dans le SpeedCache avec son nouveau rôle pour les vérifications rapides
	_ = cache_service.UpdateMemberSpeedCache(ctx, lite_models.MemberLiteRequest{
		ConversationID:    targetMem.ConversationID,
		UserID:            targetMem.UserID,
		Role:              targetMem.Role, // -4
		Settings:          service.ToMemberSettingsLite(targetMem.Settings),
		UnreadCount:       targetMem.UnreadCount,
		FrozenMessageID:   targetMem.FrozenMessageID,
		LastReadMessageID: targetMem.LastReadMessageID,
		JoinedAt:          targetMem.JoinedAt,
	})

	// 6. ENVOI AUX WORKERS (Write-Behind)
	// On envoie un Update (et non plus un Delete)
	err = redis.EnqueueDB(ctx, targetMem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, targetMem, redis.TargetAll)

	// 7. NOTIFICATION TEMPS RÉEL (WebSocket Uniquement, Zéro Push natif)
	if err == nil {
		go func() {
			bgCtx := context.Background()
			// On prévient juste l'application en sous-marin pour mettre à jour l'UI
			payload := map[string]interface{}{
				"conversation_id": input.ConversationID,
				"role":            -4,
			}
			_ = realtime_service.DistributeToUsers(bgCtx, "community_request.refused", payload, []int64{input.TargetUserID})
		}()
	}

	output := member_models.RefuseCommunityRequestOutput{}

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
