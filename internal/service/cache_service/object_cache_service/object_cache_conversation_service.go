package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
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
		logger.Log.Error().Err(errRedis).Int64("conv_id", conversationPayload.ID).Msg("Échec de l'écriture de la conversation dans l'Object Cache")
		return nubo_error.NewInternal()
	}
	return nil
}

// DeleteConversationFromObjectCache supprime une conversation du cache LFU.
func DeleteConversationFromObjectCache(ctx context.Context, conversationID int64) error {
	errRedis := redis.Conversations.DeleteObject(ctx, conversationID)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Int64("conv_id", conversationID).Msg("Échec de la suppression de la conversation de l'Object Cache")
		return nubo_error.NewInternal()
	}
	return nil
}
