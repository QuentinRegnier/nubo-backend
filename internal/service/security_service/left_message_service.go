package security_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// LeftMessage récupère un message complet (L1 -> L2 -> L3) et vérifie les droits de l'auteur.
func LeftMessage(ctx context.Context, messageID int64, userID int64) (message_models.MessagePayload, error) {
	var msg message_models.MessagePayload
	var found bool

	// 1. TENTATIVE L1 (Object Cache)
	if m, err := object_cache_service.GetMessageFromObjectCache(ctx, messageID); err == nil && m.ID != 0 {
		msg = m
		found = true
	} else {
		// 2. TENTATIVE L2 (MongoDB)
		if mongoMsgs, errMongo := mongo.MongoLoadMessagesByIDs([]int64{messageID}); errMongo == nil && len(mongoMsgs) > 0 {
			msg = mongoMsgs[0]
			found = true
			_ = object_cache_service.SetMessageInObjectCache(ctx, msg) // Auto-guérison L1
		} else {
			// 3. FALLBACK ABSOLU L3 (PostgreSQL)
			if pgMsgs, errPg := postgres.FuncLoadMessagesByIDs(ctx, []int64{messageID}); errPg == nil && len(pgMsgs) > 0 {
				msg = pgMsgs[0]
				found = true
				_ = mongo.MongoUpsertMessage(msg)                          // Auto-guérison L2
				_ = object_cache_service.SetMessageInObjectCache(ctx, msg) // Auto-guérison L1
			}
		}
	}

	// 4. VÉRIFICATION DES RÈGLES DE SÉCURITÉ
	if !found {
		return message_models.MessagePayload{}, errors.New("not found")
	}

	if !msg.Visibility {
		return message_models.MessagePayload{}, errors.New("not found") // Furtivité (Soft-delete)
	}

	if msg.SenderID != userID {
		return message_models.MessagePayload{}, errors.New("unauthorized")
	}

	return msg, nil
}
