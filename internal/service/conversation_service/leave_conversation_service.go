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
)

// LeaveConversation gère la suppression côté client (MP) et le départ (Groupe/Communauté)
func LeaveConversation(ctx context.Context, callerID int64, convID int64, input conversation_models.LeaveConversationInput) (conversation_models.LeaveConversationOutput, error) {
	// 1. SÉCURITÉ ET RÉCUPÉRATION (Objet Complet)
	mem, err := security_service.LeftMember(ctx, convID, callerID)
	if err != nil {
		return conversation_models.LeaveConversationOutput{}, err // L'erreur est déjà une AppError formatée par security_service
	}
	conv, err := object_cache_service.GetConversationFromObjectCache(ctx, convID)
	if err != nil {
		return conversation_models.LeaveConversationOutput{}, nubo_error.NewNotFound("CONV_NOT_FOUND", "Impossible de charger les détails de la conversation.", err)
	}

	// 2. GESTION DU PROPRIÉTAIRE (Type > 0)
	// On vérifie en O(1) combien de personnes actives sont dans la conversation via Redis
	participantCount, _ := redis.ConvParticipants.SCard(ctx, convID)

	if conv.Type > 0 && mem.Role == 2 && participantCount > 1 {
		if input.NewOwnerID == 0 {
			return conversation_models.LeaveConversationOutput{}, nubo_error.NewForbidden("MUST_TRANSFER_OWNERSHIP", "Vous devez transférer la propriété à un administrateur avant de quitter.", nil)
		}

		newOwnerMem, err := security_service.LeftMember(ctx, convID, input.NewOwnerID)
		if err != nil || newOwnerMem.Role != 1 {
			return conversation_models.LeaveConversationOutput{}, nubo_error.NewBadRequest("INVALID_NEW_OWNER", "Le nouveau propriétaire doit être un administrateur existant du groupe.", err)
		}

		// Promotion du nouveau propriétaire
		newOwnerMem.Role = 2
		newOwnerMem.UpdatedAt = domain.NowMillis()
		_ = object_cache_service.SetMemberInObjectCache(ctx, newOwnerMem)
		_ = redis.EnqueueDB(ctx, newOwnerMem.ID, convID, redis.EntityMembers, redis.ActionUpdate, newOwnerMem, redis.TargetAll)
	}

	// 3. APPLICATION DU DÉPART (Rôle = -1)
	mem.Role = -1
	mem.UpdatedAt = domain.NowMillis()
	_ = object_cache_service.SetMemberInObjectCache(ctx, mem)

	// === NOUVEAU : PURGE SYNCHRONE DU SPEED CACHE ===
	_ = cache_service.RemoveMemberFromSpeedCache(ctx, mem.ConversationID, mem.UserID)

	// Write-behind...
	err = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)

	// Envoie notification (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), convID, "member.left", mem)
			if err != nil {
				logger.Log.Error().Err(err).Msg("Failed to broadcast member left")
			}

			// ✅ NOUVEAU : SYNC LEDGER (Trigger global)
			participantsStr, _ := redis.ConvParticipants.SMembers(context.Background(), convID)
			var pIDs []int64
			for _, p := range participantsStr {
				if id, err := strconv.ParseInt(p, 10, 64); err == nil {
					pIDs = append(pIDs, id)
				}
			}
			// On ajoute le partant pour que son client efface localement au prochain /sync
			pIDs = append(pIDs, mem.UserID)
			_ = cache_service.RecordConversationMutation(context.Background(), convID, pIDs)
		}()
	}

	// 4. VÉRIFICATION DE L'EXTINCTION DE LA CONVERSATION
	// Si on était le dernier participant (participantCount redescend à 0 après notre départ)
	if participantCount <= 1 {
		// On attribue le bon statut selon le type
		if conv.Type == 0 {
			conv.State = -1 // MP Supprimé
		} else {
			conv.State = -2 // Groupe/Communauté Supprimé
		}
		conv.UpdatedAt = domain.NowMillis()

		// Mise à jour L1 et File Asynchrone
		_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
		_ = redis.EnqueueDB(ctx, conv.ID, conv.ID, redis.EntityConversation, redis.ActionUpdate, conv, redis.TargetAll)
	}

	output := conversation_models.LeaveConversationOutput{}
	// ========================================================================
	// 5. MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé TOUT À LA FIN de la fonction. Cela écrase tout timestamp qui aurait
	// pu être généré précédemment (par ex. à l'intérieur de AddMembersToConversation)
	// et garantit que le client reçoit la date de la fin absolue de la transaction.
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	output.InboxUpdateAt = domain.TimeToMillis(time.UnixMilli(timestampMs))

	return output, nil
}
