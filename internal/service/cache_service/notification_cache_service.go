package cache_service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : CACHE DES NOTIFICATIONS (ZSET)
// ############################################################################

// AddNotificationToZSET plafonne le ZSET à 100 éléments via script Lua et rafraîchit le TTL.
func AddNotificationToZSET(ctx context.Context, userID int64, notificationID int64, timestampMs int64) error {
	notificationIDStr := strconv.FormatInt(notificationID, 10)
	errRedis := redis.NotificationsZSet.ZAddWithCap(ctx, userID, float64(timestampMs), notificationIDStr, 100)

	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userID).Msg("Impossible d'ajouter la notification au ZSET")
		return nubo_error.NewInternal()
	}

	_ = redis.NotificationsZSet.RefreshTTL(ctx, userID)
	return nil
}

// GetNotificationIDsFromZSET récupère un batch d'IDs paginés depuis le ZSET de l'utilisateur.
func GetNotificationIDsFromZSET(ctx context.Context, userID int64, offset int64, limit int64) ([]int64, error) {
	idStringsList, errRedis := redis.NotificationsZSet.ZRevRange(ctx, userID, offset, offset+limit-1)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userID).Msg("Erreur de récupération des IDs de notification dans le ZSET")
		return nil, nubo_error.NewInternal()
	}

	var parsedNotificationIDs []int64
	for _, stringID := range idStringsList {
		if parsedID, errParse := strconv.ParseInt(stringID, 10, 64); errParse == nil {
			parsedNotificationIDs = append(parsedNotificationIDs, parsedID)
		}
	}

	// Prolongation de la vie du cache puisqu'il vient d'être accédé
	if len(parsedNotificationIDs) > 0 {
		_ = redis.NotificationsZSet.RefreshTTL(ctx, userID)
	}

	return parsedNotificationIDs, nil
}

// PurgeNotificationsZSET détruit l'intégralité du cache des notifications de l'utilisateur.
func PurgeNotificationsZSET(ctx context.Context, userID int64) error {
	errRedis := redis.NotificationsZSet.DeleteObject(ctx, userID)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Msg("Erreur lors de la purge du ZSET des notifications")
		return nubo_error.NewInternal()
	}
	return nil
}

// TouchActivityTimestamp signale une modification d'état (ex: marquage comme lu)
// et retourne le timestamp exact généré par le serveur.
func TouchActivityTimestamp(ctx context.Context, userID int64) int64 {
	currentTimestampMs := time.Now().UnixMilli()

	errRedis := redis.NotificationActivity.SetPrimitive(ctx, userID, currentTimestampMs)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Int64("user_id", userID).Msg("Échec de la mise à jour du Timestamp d'Activité (Notification)")
	}

	return currentTimestampMs
}
