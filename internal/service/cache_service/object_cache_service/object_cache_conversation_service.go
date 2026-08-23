package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// GetConversationFromObjectCache récupère le payload complet d'une conversation depuis le cache LFU
func GetConversationFromObjectCache(ctx context.Context, convID int64) (conversation_models.ConversationPayload, error) {
	var c conversation_models.ConversationPayload
	err := redis.Conversations.GetObject(ctx, convID, &c)
	return c, err
}

// SetConversationInObjectCache insère ou met à jour une conversation dans le cache LFU
func SetConversationInObjectCache(ctx context.Context, conv conversation_models.ConversationPayload) error {
	return redis.Conversations.SetObject(ctx, conv.ID, conv)
}

// DeleteConversationFromObjectCache supprime une conversation du cache LFU
func DeleteConversationFromObjectCache(ctx context.Context, convID int64) error {
	return redis.Conversations.DeleteObject(ctx, convID)
}
