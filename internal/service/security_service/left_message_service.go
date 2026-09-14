package security_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
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

			// ⬆️ Auto-Guérison L1 (Immédiat en RAM)
			_ = object_cache_service.SetMessageInObjectCache(ctx, msg)
		} else {
			// 3. FALLBACK ABSOLU L3 (PostgreSQL)
			if pgMsgs, errPg := postgres.FuncLoadMessagesByIDs(ctx, []int64{messageID}); errPg == nil && len(pgMsgs) > 0 {
				msg = pgMsgs[0]
				found = true

				// ⬆️ Auto-Guérison L1 (Immédiat en RAM)
				_ = object_cache_service.SetMessageInObjectCache(ctx, msg)

				// ⬆️ Auto-Guérison L2 (Asynchrone via Worker Mongo)
				go func(m message_models.MessagePayload) {
					// PartitionKey = ConversationID pour grouper les messages chronologiquement
					_ = redis.EnqueueDB(context.Background(), m.ID, m.ConversationID, redis.EntityMessage, redis.ActionUpdate, m, redis.TargetMongo)
				}(msg)
			}
		}
	}

	// 4. VÉRIFICATION DES RÈGLES DE SÉCURITÉ
	if !found || !msg.Visibility {
		// Furtivité absolue : on ne dit pas si le message existe mais est caché
		return message_models.MessagePayload{}, nubo_error.NewNotFound("MESSAGE_NOT_FOUND", "Message introuvable.", nil)
	}
	if msg.SenderID != userID {
		return message_models.MessagePayload{}, nubo_error.NewForbidden("ACCESS_DENIED", "Accès refusé.", nil)
	}

	return msg, nil
}
