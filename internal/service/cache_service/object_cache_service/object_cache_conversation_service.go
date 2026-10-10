package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : OBJECT CACHE (CONVERSATIONS LFU)
// ############################################################################

// GetConversationFromObjectCache récupère le payload complet d'une conversation depuis le cache LFU.
func GetConversationFromObjectCache(ctx context.Context, conversationID int64) (conversation_models.ConversationPayload, error) {
	var conversationPayload conversation_models.ConversationPayload

	errRedis := redis.Conversations.GetObject(ctx, conversationID, &conversationPayload)
	if errRedis != nil {
		return conversation_models.ConversationPayload{}, errRedis // Laisse remonter si c'est un Cache Miss (géré par la cascade)
	}

	return conversationPayload, nil
}

// SetConversationInObjectCache insère ou met à jour une conversation dans le cache LFU.
func SetConversationInObjectCache(ctx context.Context, conversationPayload conversation_models.ConversationPayload) error {
	errRedis := redis.Conversations.SetObject(ctx, conversationPayload.ID, conversationPayload)
	if errRedis != nil {
		numan_log.Error(ctx).Err(errRedis).Int64("conv_id", conversationPayload.ID).Msg("Échec de l'écriture de la conversation dans l'Object Cache")
		return numan_error.NewInternal()
	}
	return nil
}
