package cache_service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : ÉCRITURES DANS LE LEDGER DE SYNCHRONISATION (TRIGGERS)
// ############################################################################

// RecordConversationMutation enregistre une modification globale sur la conversation
// dans le Ledger de CHAQUE participant en un seul aller-retour réseau (O(1) Pipeline Redis).
// Utilisé pour les changements de nom, avatars, permissions, mute, kick...
func RecordConversationMutation(ctx context.Context, conversationID int64, participantIDsList []int64) error {

	currentTimestampMs := float64(domain.NowMillis())
	conversationIDString := strconv.FormatInt(conversationID, 10)

	// DÉLÉGATION DDD ABSOLUE : Le Cache Service ne parle qu'à l'abstraction Collection
	errRedis := redis.UserSyncLedger.ZAddMultiple(ctx, participantIDsList, currentTimestampMs, conversationIDString)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("conv_id", conversationID).Msg("Échec de la mutation du Ledger Conversation")
		return nubo_error.NewInternal()
	}

	return nil
}

// RecordMessageMutation enregistre une modification granulaire sur un message
// (Édition, Suppression, Réaction) ET signale la conversation entière comme mutée aux participants.
func RecordMessageMutation(ctx context.Context, conversationID int64, messageID int64, participantIDsList []int64) error {

	currentTimestampMs := float64(domain.NowMillis())
	messageIDString := strconv.FormatInt(messageID, 10)

	// 1. Inscription du message dans le Ledger (ZSET unique par conversation)
	errRedis := redis.ConvMessageLedger.ZAdd(ctx, conversationID, currentTimestampMs, messageIDString)
	if errRedis == nil {
		_ = redis.ConvMessageLedger.RefreshTTL(ctx, conversationID)
	} else {
		logger.Log.Warn().Err(errRedis).Int64("message_id", messageID).Msg("Échec de l'insertion dans le ConvMessageLedger")
	}

	// 2. On "allume le gyrophare" sur la conversation parente pour tous les participants
	return RecordConversationMutation(ctx, conversationID, participantIDsList)
}

// ############################################################################
// # SERVICE : LECTURES DEPUIS LE LEDGER (PULL SYNCHRONIZATION)
// ############################################################################

// GetModifiedConversationIDs retourne la liste des IDs de conversations ayant muté depuis un timestamp.
func GetModifiedConversationIDs(ctx context.Context, userID int64, sinceTimestampMs int64) ([]int64, error) {

	// Syntaxe Redis : "(" indique "strictement supérieur"
	minScoreThreshold := fmt.Sprintf("(%d", sinceTimestampMs)
	maxScoreThreshold := "+inf"

	// DÉLÉGATION DDD : O(log N) abstrait et sécurisé (limite arbitraire fixée à 1000 pour protéger la RAM)
	idStringsList, errRedis := redis.UserSyncLedger.ZRangeByScore(ctx, userID, minScoreThreshold, maxScoreThreshold, 1000)

	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userID).Msg("Erreur lors de la lecture du UserSyncLedger")
		return nil, nubo_error.NewInternal()
	}

	var parsedConversationIDs []int64
	for _, idString := range idStringsList {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			parsedConversationIDs = append(parsedConversationIDs, parsedID)
		}
	}

	return parsedConversationIDs, nil
}

// GetModifiedMessageIDs retourne la liste des IDs de messages ayant muté dans une conversation spécifique.
func GetModifiedMessageIDs(ctx context.Context, conversationID int64, sinceTimestampMs int64) ([]int64, error) {

	minScoreThreshold := fmt.Sprintf("(%d", sinceTimestampMs)
	maxScoreThreshold := "+inf"

	// DÉLÉGATION DDD
	idStringsList, errRedis := redis.ConvMessageLedger.ZRangeByScore(ctx, conversationID, minScoreThreshold, maxScoreThreshold, 1000)

	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("conv_id", conversationID).Msg("Erreur lors de la lecture du ConvMessageLedger")
		return nil, nubo_error.NewInternal()
	}

	var parsedMessageIDs []int64
	for _, idString := range idStringsList {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			parsedMessageIDs = append(parsedMessageIDs, parsedID)
		}
	}

	return parsedMessageIDs, nil
}
