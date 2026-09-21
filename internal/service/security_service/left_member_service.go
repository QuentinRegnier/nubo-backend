package security_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// LeftMember vérifie que l'utilisateur fait partie de la conversation et retourne son profil de membre complet (L1->L2->L3).
func LeftMember(ctx context.Context, convID int64, userID int64) (member_models.MemberPayload, error) {
	var mem member_models.MemberPayload
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

			// ⬆️ Auto-Guérison L1 (Immédiat en RAM)
			_ = object_cache_service.SetMemberInObjectCache(ctx, mem)

			// ⬆️ Auto-Guérison L2 (Asynchrone via Worker Mongo)
			go func(m member_models.MemberPayload) {
				_ = redis.EnqueueDB(context.Background(), m.ID, m.ConversationID, redis.EntityMembers, redis.ActionUpdate, m, redis.TargetMongo)
			}(mem)
		}
	}

	// 4. VÉRIFICATION DES RÈGLES
	if !found {
		return member_models.MemberPayload{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Accès refusé : vous n'êtes pas membre de cette conversation.", nil)
	}
	if mem.Role < 0 {
		return member_models.MemberPayload{}, nubo_error.NewForbidden("USER_BANNED", "Accès refusé : vous êtes banni de cette conversation.", nil)
	}

	return mem, nil
}
