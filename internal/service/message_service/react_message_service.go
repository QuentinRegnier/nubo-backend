package message_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/worker"
)

// ReactToMessage gère l'ajout ou la modification d'une réaction sur un message via le Hash Cache.
func ReactToMessage(ctx context.Context, callerID int64, input message_models.ReactMessageInput) error {
	// 1. SÉCURITÉ : Récupération du message et vérification d'appartenance
	msg, err := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if err != nil {
		return err
	}
	mem, err := security_service.LeftMember(ctx, msg.ConversationID, callerID)
	if err != nil || mem.Role < 0 {
		return nubo_error.NewForbidden("ACCESS_DENIED", "Accès refusé : vous ne faites pas partie de cette conversation.", err)
	}

	input.Reaction = pkg.CleanStr(input.Reaction)

	// 2. VÉRIFICATION IDEMPOTENCE (RAM L1)
	oldEmoji, _ := cache_service.GetUserReaction(ctx, msg.ID, callerID)

	if oldEmoji == input.Reaction {
		return nil // Idempotence parfaite : l'utilisateur a cliqué sur le même emoji
	}

	// 3. APPLICATION DU DELTA (RAM L1 & WORKER BATCH)
	if oldEmoji != "" {
		_ = cache_service.IncrementReactionCount(ctx, msg.ID, oldEmoji, -1)
		worker.RegisterMessageReaction(msg.ID, oldEmoji, -1) // ✅ NOUVEAU
	}

	_ = cache_service.IncrementReactionCount(ctx, msg.ID, input.Reaction, 1)
	worker.RegisterMessageReaction(msg.ID, input.Reaction, 1) // ✅ NOUVEAU

	_ = cache_service.SetUserReaction(ctx, msg.ID, callerID, input.Reaction)

	// 4. PERSISTANCE ASYNCHRONE (Write-Behind vers MongoDB/Postgres)
	payload := message_models.MessageReactionPayload{
		ID:        pkg.GenerateID(),
		MessageID: msg.ID,
		UserID:    callerID,
		Reaction:  input.Reaction,
		CreatedAt: service.NowMillis(),
	}

	// L'ActionCreate déclenchera l'UPSERT côté Worker grâce à la contrainte UNIQUE SQL
	err = redis.EnqueueDB(ctx, payload.ID, msg.ConversationID, redis.EntityMessageReaction, redis.ActionCreate, payload, redis.TargetAll)

	// 5. DIFFUSION TEMPS RÉEL (WebSockets)
	if err == nil {
		go func() {
			bgCtx := context.Background()
			counts, _ := cache_service.GetMessageReactionCounts(bgCtx, msg.ID)

			msgView := message_models.MessageView{
				MessagePayload: msg,
				ReactionCounts: counts,
				// UserReaction n'est pas envoyé en broadcast car spécifique à chaque receveur
			}

			errWs := realtime_service.BroadcastToConversation(bgCtx, msg.ConversationID, "message.reacted", msgView)
			if errWs != nil {
				logger.Log.Error().Err(errWs).Msg("Failed to broadcast message reaction")
			}
		}()
	}

	return err
}
