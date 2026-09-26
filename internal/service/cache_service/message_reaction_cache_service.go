package cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : CACHE DES RÉACTIONS DE MESSAGERIE (HASH)
// ############################################################################

// GetUserReaction récupère l'emoji spécifique posé par un utilisateur sur un message.
func GetUserReaction(ctx context.Context, messageID int64, userID int64) (string, error) {
	emojiString, errRedis := redis.MessageUserReactions.HGet(ctx, messageID, strconv.FormatInt(userID, 10)).Result()

	if errRedis != nil {
		// Redis renvoie une erreur s'il n'y a pas de résultat, on l'étouffe proprement si c'est un "Nil".
		return "", nil
	}

	return emojiString, nil
}

// SetUserReaction sauvegarde en RAM (O(1)) l'emoji choisi par l'utilisateur.
func SetUserReaction(ctx context.Context, messageID int64, userID int64, emojiString string) error {
	errRedis := redis.MessageUserReactions.HSet(ctx, messageID, strconv.FormatInt(userID, 10), emojiString)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("message_id", messageID).Msg("Impossible de sauvegarder la réaction utilisateur dans le cache L1")
		return nubo_error.NewInternal()
	}
	return nil
}

// DeleteUserReaction supprime la trace de la réaction de l'utilisateur.
func DeleteUserReaction(ctx context.Context, messageID int64, userID int64) error {
	errRedis := redis.MessageUserReactions.HDel(ctx, messageID, strconv.FormatInt(userID, 10))
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("message_id", messageID).Msg("Impossible de supprimer la réaction utilisateur du cache L1")
		return nubo_error.NewInternal()
	}
	return nil
}

// IncrementReactionCount modifie le compteur aggloméré d'un emoji spécifique.
func IncrementReactionCount(ctx context.Context, messageID int64, emojiString string, delta int64) error {
	redisKey := redis.MessageReactionCounts.Key(messageID)

	newCounterValue, errRedis := redis.MessageReactionCounts.Client.HIncrBy(ctx, redisKey, emojiString, delta).Result()
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("message_id", messageID).Msg("Échec de l'incrémentation du compteur de réaction dans Redis")
		return nubo_error.NewInternal()
	}

	// Nettoyage automatique RAM si le compteur tombe à zéro ou en négatif pour ne pas polluer le Payload JSON.
	if newCounterValue <= 0 {
		errDel := redis.MessageReactionCounts.Client.HDel(ctx, redisKey, emojiString).Err()
		if errDel != nil {
			logger.Log.Warn().Err(errDel).Msg("Échec du nettoyage HDel après décrémentation à zéro")
		}
	}

	return nil
}

// GetMessageReactionCounts récupère l'ensemble des compteurs agglomérés pour un message (O(1)).
func GetMessageReactionCounts(ctx context.Context, messageID int64) (map[string]int, error) {
	rawReactionMap, errRedis := redis.MessageReactionCounts.HGetAll(ctx, messageID).Result()

	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("message_id", messageID).Msg("Échec de la récupération des compteurs de réactions")
		return nil, nubo_error.NewInternal()
	}

	reactionCounts := make(map[string]int)
	for emojiKey, countStr := range rawReactionMap {
		if parsedCount, errParse := strconv.Atoi(countStr); errParse == nil && parsedCount > 0 {
			reactionCounts[emojiKey] = parsedCount
		}
	}

	return reactionCounts, nil
}
