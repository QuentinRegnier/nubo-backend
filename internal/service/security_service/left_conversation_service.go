package security_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// LeftConversation vérifie les droits d'administration et retourne la conversation complète.
func LeftConversation(ctx context.Context, convID int64, userID int64) (conversation_models.ConversationPayload, error) {
	var conv conversation_models.ConversationPayload
	var mem member_models.MemberPayload
	var foundConv, foundMem bool

	// 1. TENTATIVE L1 (OBJECT CACHE - LFU)
	if c, err := object_cache_service.GetConversationFromObjectCache(ctx, convID); err == nil && c.ID != 0 {
		conv = c
		foundConv = true
	}
	if m, err := object_cache_service.GetMemberFromObjectCache(ctx, convID, userID); err == nil && m.ID != 0 {
		mem = m
		foundMem = true
	}

	// 2. CASCADE L2 (MONGODB)
	if !foundConv {
		if mongoConv, err := mongo.MongoGetConversation(convID); err == nil && mongoConv.ID != 0 {
			conv = mongoConv
			foundConv = true
			_ = object_cache_service.SetConversationInObjectCache(ctx, conv) // Auto-Guérison L1
		}
	}
	if !foundMem {
		if mongoMem, err := mongo.MongoGetMember(convID, userID); err == nil && mongoMem.ID != 0 {
			mem = mongoMem
			foundMem = true
			_ = object_cache_service.SetMemberInObjectCache(ctx, mem) // Auto-Guérison L1
		}
	}

	// 3. CASCADE ABSOLUE L3 (POSTGRESQL)
	if !foundConv {
		if pgConv, err := postgres.FuncGetConversation(ctx, convID); err == nil && pgConv.ID != 0 {
			conv = pgConv
			foundConv = true

			// ⬆️ Auto-Guérison L1 (Immédiat en RAM)
			_ = object_cache_service.SetConversationInObjectCache(ctx, conv)

			// ⬆️ Auto-Guérison L2 (Asynchrone via Worker Mongo)
			go func(c conversation_models.ConversationPayload) {
				_ = redis.EnqueueDB(context.Background(), c.ID, c.ID, redis.EntityConversation, redis.ActionUpdate, c, redis.TargetMongo)
			}(conv)
		}
	}
	if !foundMem {
		if pgMem, err := postgres.FuncGetMember(ctx, convID, userID); err == nil && pgMem.ID != 0 {
			mem = pgMem
			foundMem = true

			// ⬆️ Auto-Guérison L1 (Immédiat en RAM)
			_ = object_cache_service.SetMemberInObjectCache(ctx, mem)

			// ⬆️ Auto-Guérison L2 (Asynchrone via Worker Mongo)
			go func(m member_models.MemberPayload) {
				// PartitionKey = ConversationID pour les membres
				_ = redis.EnqueueDB(context.Background(), m.ID, m.ConversationID, redis.EntityMembers, redis.ActionUpdate, m, redis.TargetMongo)
			}(mem)
		}
	}

	// 4. VÉRIFICATION DES RÈGLES MÉTIER ET DE SÉCURITÉ
	if !foundConv || conv.State == 1 {
		return conversation_models.ConversationPayload{}, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation introuvable ou archivée.", nil)
	}
	if !foundMem {
		return conversation_models.ConversationPayload{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Accès refusé : vous n'êtes pas membre de cette conversation.", nil)
	}
	// Le rôle doit être au moins Admin (1) ou Propriétaire (2) pour modifier la conversation
	if mem.Role < 1 {
		return conversation_models.ConversationPayload{}, nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Accès refusé : droits d'administration requis.", nil)
	}

	return conv, nil
}
