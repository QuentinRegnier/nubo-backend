package message_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/QuentinRegnier/nubo-backend/internal/worker"
)

// ############################################################################
// # SERVICE : AJOUTER OU MODIFIER UNE RÉACTION (EMOJI)
// ############################################################################

// ReactToMessage gère l'ajout ou la modification d'une réaction sur un message via le Hash Cache L1.
func ReactToMessage(ctx context.Context, callerID int64, input message_models.ReactMessageInput) error {

	// ── ÉTAPE 1 : CONTRÔLE D'ACCÈS AU MESSAGE (ZERO-TRUST) ──────────────────

	messagePayload, errSecurityMsg := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if errSecurityMsg != nil {
		return errSecurityMsg
	}

	callerMemberPayload, errSecurityMem := security_service.LeftMember(ctx, messagePayload.ConversationID, callerID)
	if errSecurityMem != nil || callerMemberPayload.Role < variables.MemberRoleNormal {
		return nubo_error.NewForbidden(nubo_error.CodeForbidden, "Accès refusé : vous ne faites pas partie de cette conversation.", errSecurityMem)
	}

	sanitizedEmoji := pkg.CleanStr(input.Reaction)

	// ── ÉTAPE 2 : VÉRIFICATION IDEMPOTENCE (RAM L1) ─────────────────────────

	previousEmoji, _ := cache_service.GetUserReaction(ctx, messagePayload.ID, callerID)
	if previousEmoji == sanitizedEmoji {
		return nil // Idempotence : L'utilisateur a cliqué sur le même emoji, succès silencieux.
	}

	// ── ÉTAPE 3 : APPLICATION DU DELTA (RAM L1 & WORKER BATCH) ──────────────

	// Si l'utilisateur avait une ancienne réaction, on la décrémente d'abord.
	if previousEmoji != "" {
		_ = cache_service.IncrementReactionCount(ctx, messagePayload.ID, previousEmoji, -1)
		worker.RegisterMessageReaction(messagePayload.ID, previousEmoji, -1)
	}

	// Incrémentation de la nouvelle réaction
	_ = cache_service.IncrementReactionCount(ctx, messagePayload.ID, sanitizedEmoji, 1)
	worker.RegisterMessageReaction(messagePayload.ID, sanitizedEmoji, 1)

	// Sauvegarde de l'état personnel de l'utilisateur
	_ = cache_service.SetUserReaction(ctx, messagePayload.ID, callerID, sanitizedEmoji)

	// ── ÉTAPE 4 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────

	reactionPayload := message_models.MessageReactionPayload{
		ID:        pkg.GenerateID(),
		MessageID: messagePayload.ID,
		UserID:    callerID,
		Reaction:  sanitizedEmoji,
		CreatedAt: domain.NowMillis(),
	}

	// L'ActionCreate déclenchera un UPSERT côté Worker grâce à la contrainte UNIQUE SQL (message_id, user_id)
	errQueue := redis.EnqueueDB(ctx, reactionPayload.ID, messagePayload.ConversationID, redis.EntityMessageReaction, redis.ActionCreate, reactionPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("message_id", messagePayload.ID).Msg("Échec du Write-Behind pour ReactToMessage")
		return nubo_error.NewInternal()
	}

	// ── ÉTAPE 5 : DIFFUSION TEMPS RÉEL (WEBSOCKETS) ─────────────────────────

	go func() {
		backgroundCtx := context.Background()
		reactionCountsMap, _ := cache_service.GetMessageReactionCounts(backgroundCtx, messagePayload.ID)

		messageViewDto := message_models.MessageView{
			MessagePayload: messagePayload,
			ReactionCounts: reactionCountsMap,
			// UserReaction n'est pas envoyé en broadcast (spécifique à chaque client)
		}

		errBroadcast := realtime_service.BroadcastToConversation(backgroundCtx, messagePayload.ConversationID, "message.reacted", messageViewDto)
		if errBroadcast != nil {
			logger.Log.Error().Err(errBroadcast).Msg("Échec de la diffusion WebSocket pour message.reacted")
		}

		// SYNC LEDGER : Trigger granulaire pour mettre à jour la base SQLite des clients
		participantIDs := GetParticipantIDsForLedgerSync(backgroundCtx, messagePayload.ConversationID)
		_ = cache_service.RecordMessageMutation(backgroundCtx, messagePayload.ConversationID, messagePayload.ID, participantIDs)
	}()

	return nil
}
