package conversation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetOrCreateDirectConversation gère la cascade L1->L2->L3 pour trouver un MP existant, ou le créer.
func GetOrCreateDirectConversation(ctx context.Context, callerID, targetID int64) (int64, error) {
	// 1. TENTATIVE L1 (Speed Cache O(n))
	if convID, err := cache_service.GetDirectConversationCache(ctx, callerID, targetID); err == nil && convID > 0 {
		return convID, nil
	}

	// 2. TENTATIVE L2 (MongoDB)
	if conv, err := mongo.MongoGetDirectConversation(callerID, targetID); err == nil && conv.ID > 0 {
		hydrateConversationCascade(ctx, conv, callerID, targetID, false)
		return conv.ID, nil
	}

	// 3. TENTATIVE L3 (PostgreSQL)
	if conv, err := postgres.FuncGetDirectConversation(ctx, callerID, targetID); err == nil && conv.ID > 0 {
		hydrateConversationCascade(ctx, conv, callerID, targetID, true)
		return conv.ID, nil
	}

	// 4. CRÉATION (Si introuvable sur toute la ligne)
	input := conversation_models.CreateConversationInput{
		Type:           0, // 0 = Message Privé
		ParticipantIDs: []int64{targetID},
	}

	// ✅ Extraction de l'ID depuis le nouvel Output de CreateConversation
	output, err := CreateConversation(ctx, callerID, input)
	if err != nil {
		return 0, err
	}

	return output.ConversationID, nil
}

// hydrateConversationCascade gère l'auto-guérison croisée des caches suite à un fallback L2/L3
func hydrateConversationCascade(ctx context.Context, conv conversation_models.ConversationPayload, u1, u2 int64, fromL3 bool) {
	if fromL3 {
		_ = mongo.MongoUpsertConversation(conv)
	}

	// Guérison de l'Object Cache L1 et des Metas
	_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
	convLite := lite_models.ConvLiteRequest{
		ID:            conv.ID,
		Type:          conv.Type,
		Title:         conv.Title,
		Description:   conv.Description, // NOUVEAU
		AvatarID:      conv.AvatarID,    // NOUVEAU
		LastMessageID: conv.LastMessageID,
	}
	_ = redis.ConvMeta.SetObject(ctx, conv.ID, convLite)

	// Guérison des Membres
	hydrateMember(ctx, conv.ID, u1, fromL3)
	hydrateMember(ctx, conv.ID, u2, fromL3)

	// Guérison du ZSET Inbox de l'utilisateur via la couche d'abstraction (DDD)
	_ = cache_service.AddConversationToUserInbox(ctx, u1, conv.ID, conv.LastMessageID)
	_ = cache_service.AddConversationToUserInbox(ctx, u2, conv.ID, conv.LastMessageID)
}

// hydrateMember gère l'auto-guérison croisée d'un membre
func hydrateMember(ctx context.Context, convID, userID int64, fromL3 bool) {
	var mem conversation_models.MemberPayload
	var err error
	if fromL3 {
		mem, err = postgres.FuncGetMember(ctx, convID, userID)
		if err == nil {
			_ = mongo.MongoUpsertMember(mem)
		}
	} else {
		mem, err = mongo.MongoGetMember(convID, userID)
	}

	if err == nil && mem.ID != 0 {
		_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
		_ = cache_service.AddMemberToSpeedCache(ctx, lite_models.MemberLiteRequest{
			ConversationID: mem.ConversationID,
			UserID:         mem.UserID,
			Role:           mem.Role,
			Settings:       service.ToMemberSettingsLite(mem.Settings),
			UnreadCount:    mem.UnreadCount,
			JoinedAt:       mem.JoinedAt.UnixMilli(),
		})
	}
}
