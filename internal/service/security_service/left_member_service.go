package security_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// LeftMember vérifie que l'utilisateur fait partie de la conversation et retourne son profil de membre complet (L1->L2->L3).
func LeftMember(ctx context.Context, convID int64, userID int64) (conversation_models.MemberPayload, error) {
	var mem conversation_models.MemberPayload
	var found bool

	// 1. TENTATIVE L1 (OBJECT CACHE - LFU)
	if m, err := object_cache_service.GetMemberFromObjectCache(ctx, convID, userID); err == nil && m.ID != 0 {
		mem = m
		found = true
	}

	// 2. CASCADE L2 (MONGODB)
	if !found {
		if mongoMem, err := mongo.MongoGetMember(convID, userID); err == nil && mongoMem.ID != 0 {
			mem = mongoMem
			found = true
			_ = object_cache_service.SetMemberInObjectCache(ctx, mem) // Auto-Guérison L1
		}
	}

	// 3. CASCADE ABSOLUE L3 (POSTGRESQL)
	if !found {
		if pgMem, err := postgres.FuncGetMember(ctx, convID, userID); err == nil && pgMem.ID != 0 {
			mem = pgMem
			found = true
			_ = mongo.MongoUpsertMember(mem)                          // Auto-Guérison L2
			_ = object_cache_service.SetMemberInObjectCache(ctx, mem) // Auto-Guérison L1
		}
	}

	// 4. VÉRIFICATION DES RÈGLES
	if !found {
		return conversation_models.MemberPayload{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Accès refusé : vous n'êtes pas membre de cette conversation.", nil)
	}
	if mem.Role < 0 {
		return conversation_models.MemberPayload{}, nubo_error.NewForbidden("USER_BANNED", "Accès refusé : vous êtes banni de cette conversation.", nil)
	}

	return mem, nil
}
