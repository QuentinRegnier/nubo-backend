package cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : SPEED CACHE (CURSEURS DE LECTURE / WATERMARKS)
// ############################################################################

// SetWatermarkInSpeedCache met à jour le curseur de lecture d'un utilisateur
// dans une conversation (O(1)).
func SetWatermarkInSpeedCache(ctx context.Context, conversationID int64, userID int64, lastReadMessageID int64) error {

	// Le HSET crée la clé si elle n'existe pas, ou met à jour le champ (userID) avec la nouvelle valeur.
	errRedis := redis.ConvWatermarks.HSet(ctx, conversationID, strconv.FormatInt(userID, 10), lastReadMessageID)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("conv_id", conversationID).Msg("Impossible de mettre à jour le watermark dans le Speed Cache L1")
		return nubo_error.NewInternal()
	}

	// On prolonge la durée de vie de cette structure en RAM à chaque activité
	_ = redis.ConvWatermarks.RefreshTTL(ctx, conversationID)

	return nil
}

// GetWatermarksFromSpeedCache récupère tous les curseurs de lecture d'une conversation d'un seul coup (O(1)).
func GetWatermarksFromSpeedCache(ctx context.Context, conversationID int64) (map[int64]int64, error) {

	rawWatermarksMap, errRedis := redis.ConvWatermarks.HGetAll(ctx, conversationID).Result()
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("conv_id", conversationID).Msg("Erreur L1 lors de la récupération des watermarks de la conversation")
		return nil, nubo_error.NewInternal()
	}

	parsedWatermarksMap := make(map[int64]int64)
	for userIDString, messageIDString := range rawWatermarksMap {
		parsedUserID, errParseUser := strconv.ParseInt(userIDString, 10, 64)
		parsedMessageID, errParseMsg := strconv.ParseInt(messageIDString, 10, 64)

		if errParseUser == nil && errParseMsg == nil {
			parsedWatermarksMap[parsedUserID] = parsedMessageID
		}
	}

	// Si des données sont présentes, on maintient le cache en vie (LFU/LRU behavior)
	if len(parsedWatermarksMap) > 0 {
		_ = redis.ConvWatermarks.RefreshTTL(ctx, conversationID)
	}

	return parsedWatermarksMap, nil
}
