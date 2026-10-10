package realtime_service

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/repository/redis"
	"github.com/QuentinRegnier/numan-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/numan-backend/internal/variables"
)

// ############################################################################
// # UTILITAIRES : ROUTAGE WEBSOCKET (FAN-OUT VS TWITCH)
// ############################################################################

// BroadcastToConversation est le routeur intelligent qui décide du mode de diffusion
// en fonction du type de conversation (Mode Twitch pour Communautés, Mode Fan-Out pour Groupes).
func BroadcastToConversation(ctx context.Context, conversationID int64, eventType string, eventPayload any) error {

	conversationPayload, errCache := object_cache_service.GetConversationFromObjectCache(ctx, conversationID)
	if errCache != nil {
		return errCache
	}

	// ── ROUTAGE : MODE TWITCH (COMMUNAUTÉS PRIVÉES ET PUBLIQUES) ────────────
	if conversationPayload.Type == variables.ConversationTypeCommunityPriv || conversationPayload.Type == variables.ConversationTypeCommunityPub {
		return DistributeToCommunity(ctx, eventType, eventPayload, conversationID)
	}

	// ── ROUTAGE : MODE FAN-OUT (MESSAGES PRIVÉS ET GROUPES) ─────────────────
	// On récupère les membres instantanément en RAM (O(1))
	participantsStringList, errRedis := redis.ConvParticipants.SMembers(ctx, conversationID)
	if errRedis != nil || len(participantsStringList) == 0 {
		return nil
	}

	targetUserIDs := pkg.ParseInt64List(participantsStringList)

	return DistributeToUsers(ctx, eventType, eventPayload, targetUserIDs)
}
