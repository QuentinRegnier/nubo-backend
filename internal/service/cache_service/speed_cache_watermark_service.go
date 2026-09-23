package cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// SetWatermarkInSpeedCache met à jour le curseur de lecture d'un utilisateur dans une conversation (O(1)).
func SetWatermarkInSpeedCache(ctx context.Context, convID int64, userID int64, lastReadMessageID int64) error {
	// Le HSET crée la clé si elle n'existe pas, ou met à jour le champ (userID) avec la nouvelle valeur.
	err := redis.ConvWatermarks.HSet(ctx, convID, strconv.FormatInt(userID, 10), lastReadMessageID)

	// On prolonge la durée de vie de cette structure en RAM à chaque activité
	if err == nil {
		_ = redis.ConvWatermarks.RefreshTTL(ctx, convID)
	}
	return err
}

// GetWatermarksFromSpeedCache récupère tous les curseurs de lecture d'une conversation d'un seul coup (O(1)).
func GetWatermarksFromSpeedCache(ctx context.Context, convID int64) (map[int64]int64, error) {
	rawMap, err := redis.ConvWatermarks.HGetAll(ctx, convID).Result()
	if err != nil {
		return nil, err
	}

	watermarks := make(map[int64]int64)
	for userIDStr, msgIDStr := range rawMap {
		uID, errU := strconv.ParseInt(userIDStr, 10, 64)
		mID, errM := strconv.ParseInt(msgIDStr, 10, 64)

		if errU == nil && errM == nil {
			watermarks[uID] = mID
		}
	}

	// Si des données sont présentes, on maintient le cache en vie (LFU/LRU behavior)
	if len(watermarks) > 0 {
		_ = redis.ConvWatermarks.RefreshTTL(ctx, convID)
	}

	return watermarks, nil
}
