package security_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// LeftConversation vérifie les droits d'administration et retourne la conversation complète.
func LeftConversation(ctx context.Context, convID int64, userID int64) (conversation_models.ConversationPayload, error) {
	var conv conversation_models.ConversationPayload
	var mem conversation_models.MemberPayload
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
			_ = mongo.MongoUpsertConversation(conv)                          // Auto-Guérison L2
			_ = object_cache_service.SetConversationInObjectCache(ctx, conv) // Auto-Guérison L1
		}
	}
	if !foundMem {
		if pgMem, err := postgres.FuncGetMember(ctx, convID, userID); err == nil && pgMem.ID != 0 {
			mem = pgMem
			foundMem = true
			_ = mongo.MongoUpsertMember(mem)                          // Auto-Guérison L2
			_ = object_cache_service.SetMemberInObjectCache(ctx, mem) // Auto-Guérison L1
		}
	}

	// 4. VÉRIFICATION DES RÈGLES MÉTIER ET DE SÉCURITÉ
	if !foundConv || conv.State == 1 {
		return conversation_models.ConversationPayload{}, errors.New("conversation introuvable ou archivée")
	}
	if !foundMem {
		return conversation_models.ConversationPayload{}, errors.New("accès refusé: vous n'êtes pas membre de cette conversation")
	}
	// Le rôle doit être au moins Admin (1) ou Propriétaire (2) pour modifier la conversation
	if mem.Role < 1 {
		return conversation_models.ConversationPayload{}, errors.New("accès refusé: droits d'administration requis")
	}

	return conv, nil
}
