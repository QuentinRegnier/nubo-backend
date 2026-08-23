package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

// GetMessageFromObjectCache récupère le payload complet d'un message depuis le cache LFU
func GetMessageFromObjectCache(ctx context.Context, messageID int64) (message_models.MessagePayload, error) {
	var m message_models.MessagePayload
	err := redis.Messages.GetObject(ctx, messageID, &m)
	return m, err
}

// SetMessageInObjectCache insère ou met à jour un message dans le cache LFU
func SetMessageInObjectCache(ctx context.Context, msg message_models.MessagePayload) error {
	return redis.Messages.SetObject(ctx, msg.ID, msg)
}

// DeleteMessageFromObjectCache supprime un message du cache LFU
func DeleteMessageFromObjectCache(ctx context.Context, messageID int64) error {
	return redis.Messages.DeleteObject(ctx, messageID)
}

// GetMessagesView : Le Pipeline d'Hydratation Optimisé pour les Messages (L1 -> L2 -> L3)
func GetMessagesView(ctx context.Context, ids []int64) ([]message_models.MessagePayload, error) {
	if len(ids) == 0 {
		return []message_models.MessagePayload{}, nil
	}

	finalMessages := make([]message_models.MessagePayload, 0, len(ids))
	tempMap := make(map[int64]message_models.MessagePayload)

	// 1. NIVEAU 1 : REDIS MGET (Ultra Rapide)
	result, err := redis.Messages.GetMany(ctx, ids)
	if err != nil {
		result = &redis.GetManyResult{MissingIDs: ids}
	} else {
		for id, data := range result.Found {
			var m message_models.MessagePayload
			// Désérialisation binaire MsgPack réelle
			if errDecode := msgpack.Unmarshal(data, &m); errDecode == nil {
				tempMap[id] = m
			} else {
				// Si l'objet en RAM est corrompu, on l'ajoute aux MissingIDs pour le forcer à se réhydrater depuis L2/L3
				result.MissingIDs = append(result.MissingIDs, id)
			}
		}
	}

	// 2. NIVEAU 2 : MONGO FALLBACK
	var stillMissingIDs []int64
	if len(result.MissingIDs) > 0 {
		mongoMessages, err := mongo.MongoLoadMessagesByIDs(result.MissingIDs)
		if err == nil {
			mongoFound := make(map[int64]bool)
			for _, m := range mongoMessages {
				tempMap[m.ID] = m
				mongoFound[m.ID] = true

				// PROMOTION L2 -> L1
				go func(msg message_models.MessagePayload) {
					_ = SetMessageInObjectCache(context.Background(), msg)
				}(m)
			}
			for _, id := range result.MissingIDs {
				if !mongoFound[id] {
					stillMissingIDs = append(stillMissingIDs, id)
				}
			}
		} else {
			stillMissingIDs = result.MissingIDs
		}
	}

	// 3. NIVEAU 3 : POSTGRES FALLBACK
	if len(stillMissingIDs) > 0 {
		pgMessages, err := postgres.FuncLoadMessagesByIDs(ctx, stillMissingIDs)
		if err == nil {
			for _, m := range pgMessages {
				tempMap[m.ID] = m

				// PROMOTION L3 -> L2 & L1
				go func(msg message_models.MessagePayload) {
					_ = mongo.MongoUpsertMessage(msg)
					_ = SetMessageInObjectCache(context.Background(), msg)
				}(m)
			}
		}
	}

	// ASSEMBLAGE FINAL
	for _, id := range ids {
		if m, ok := tempMap[id]; ok && m.Visibility {
			finalMessages = append(finalMessages, m)
		}
	}
	return finalMessages, nil
}
