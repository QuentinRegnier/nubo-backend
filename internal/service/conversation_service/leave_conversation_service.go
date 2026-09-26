package conversation_service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : QUITTER UNE CONVERSATION (OU SUPPRIMER UN MP)
// ############################################################################

// LeaveConversation gère la suppression locale (MP) et le départ (Groupe/Communauté),
// incluant le transfert de propriété obligatoire pour l'Owner.
func LeaveConversation(ctx context.Context, callerID int64, conversationID int64, input conversation_models.LeaveConversationInput) (conversation_models.LeaveConversationOutput, error) {

	// ── ÉTAPE 1 : CONTRÔLE D'ACCÈS ET RÉCUPÉRATION L1 ───────────────────────
	// LeftMember renvoie déjà une AppError formatée si nécessaire
	memberPayload, errSecurity := security_service.LeftMember(ctx, conversationID, callerID)
	if errSecurity != nil {
		return conversation_models.LeaveConversationOutput{}, errSecurity
	}

	conversationPayload, errCache := object_cache_service.GetConversationFromObjectCache(ctx, conversationID)
	if errCache != nil || conversationPayload.ID == 0 {
		return conversation_models.LeaveConversationOutput{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Impossible de charger les détails de la conversation.", errCache)
	}

	// ── ÉTAPE 2 : GESTION DU PROPRIÉTAIRE (GROUPES & COMMUNAUTÉS) ───────────
	// Lecture O(1) du nombre de participants actifs dans la RAM
	activeParticipantCount, _ := redis.ConvParticipants.SCard(ctx, conversationID)

	if conversationPayload.Type > variables.ConversationTypeDirect && memberPayload.Role == variables.MemberRoleOwner && activeParticipantCount > 1 {
		if input.NewOwnerID == 0 {
			return conversation_models.LeaveConversationOutput{}, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Vous devez impérativement transférer la propriété à un administrateur avant de quitter le groupe.", nil)
		}

		newOwnerPayload, errOwnerSecurity := security_service.LeftMember(ctx, conversationID, input.NewOwnerID)
		if errOwnerSecurity != nil || newOwnerPayload.Role != variables.MemberRoleAdmin {
			return conversation_models.LeaveConversationOutput{}, nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Le nouveau propriétaire désigné doit être un administrateur actif du groupe.", errOwnerSecurity)
		}

		// Promotion du successeur
		newOwnerPayload.Role = variables.MemberRoleOwner
		newOwnerPayload.UpdatedAt = domain.NowMillis()

		_ = object_cache_service.SetMemberInObjectCache(ctx, newOwnerPayload)
		errQueueOwner := redis.EnqueueDB(ctx, newOwnerPayload.ID, conversationID, redis.EntityMembers, redis.ActionUpdate, newOwnerPayload, redis.TargetAll)
		if errQueueOwner != nil {
			logger.Log.Error().Err(errQueueOwner).Int64("user_id", newOwnerPayload.UserID).Msg("Échec d'enqueue de la promotion du propriétaire")
		}
	}

	// ── ÉTAPE 3 : APPLICATION DU DÉPART (SOFT DELETE) ───────────────────────
	memberPayload.Role = variables.MemberRoleLeft
	memberPayload.UpdatedAt = domain.NowMillis()

	_ = object_cache_service.SetMemberInObjectCache(ctx, memberPayload)
	_ = cache_service.RemoveMemberFromSpeedCache(ctx, memberPayload.ConversationID, memberPayload.UserID)

	errQueueLeave := redis.EnqueueDB(ctx, memberPayload.ID, conversationID, redis.EntityMembers, redis.ActionUpdate, memberPayload, redis.TargetAll)
	if errQueueLeave != nil {
		logger.Log.Error().Err(errQueueLeave).Int64("user_id", memberPayload.UserID).Msg("Échec d'enqueue du départ de l'utilisateur")
		return conversation_models.LeaveConversationOutput{}, nubo_error.NewInternal()
	}

	// ── ÉTAPE 4 : NOTIFICATION ET SYNCHRONISATION TEMPS RÉEL ────────────────
	go func() {
		backgroundContext := context.Background()

		errBroadcast := realtime_service.BroadcastToConversation(backgroundContext, conversationID, "member.left", memberPayload)
		if errBroadcast != nil {
			logger.Log.Error().Err(errBroadcast).Msg("Échec de la diffusion WebSocket pour member.left")
		}

		// SYNC LEDGER : On avertit tous les autres participants qu'une mutation globale a eu lieu
		participantsStringList, _ := redis.ConvParticipants.SMembers(backgroundContext, conversationID)
		var syncTargetIDs []int64
		for _, participantStr := range participantsStringList {
			if parsedID, errParse := strconv.ParseInt(participantStr, 10, 64); errParse == nil {
				syncTargetIDs = append(syncTargetIDs, parsedID)
			}
		}
		// On ajoute l'utilisateur qui vient de partir pour forcer son client à effacer la conv localement au prochain /sync
		syncTargetIDs = append(syncTargetIDs, memberPayload.UserID)

		_ = cache_service.RecordConversationMutation(backgroundContext, conversationID, syncTargetIDs)
	}()

	// ── ÉTAPE 5 : EXTINCTION ALGORITHMIQUE DE LA CONVERSATION ───────────────
	// Si le partant était l'unique participant restant
	if activeParticipantCount <= 1 {
		if conversationPayload.Type == variables.ConversationTypeDirect {
			conversationPayload.State = variables.ConversationStatePrivateDelete
		} else {
			conversationPayload.State = variables.ConversationStateGroupDelete
		}
		conversationPayload.UpdatedAt = domain.NowMillis()

		_ = object_cache_service.SetConversationInObjectCache(ctx, conversationPayload)
		_ = redis.EnqueueDB(ctx, conversationPayload.ID, conversationPayload.ID, redis.EntityConversation, redis.ActionUpdate, conversationPayload, redis.TargetAll)
	}

	// ── ÉTAPE 6 : MARQUAGE D'ACTIVITÉ (DIRTY FLAG) ──────────────────────────
	latestActivityTimestampMs := cache_service.TouchInboxActivity(ctx, callerID)

	return conversation_models.LeaveConversationOutput{
		InboxUpdateAt: domain.TimeToMillis(time.UnixMilli(latestActivityTimestampMs)),
	}, nil
}
