package cache_service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ============================================================================
// 1. ÉCRITURES DANS LE LEDGER (TRIGGERS)
// ============================================================================

// RecordConversationMutation enregistre une modification globale sur la conversation
// dans le Ledger de CHAQUE participant en un seul aller-retour réseau (O(1) Pipeline).
// Utilisé pour: changement de nom, avatar, permissions, nouveaux membres, mutes, etc.
func RecordConversationMutation(ctx context.Context, convID int64, participantIDs []int64) error {
	nowMs := float64(domain.NowMillis())
	convIDStr := strconv.FormatInt(convID, 10)

	// DÉLÉGATION DDD ABSOLUE : Le Cache Service ne parle qu'à l'abstraction Collection
	return redis.UserSyncLedger.ZAddMultiple(ctx, participantIDs, nowMs, convIDStr)
}

// RecordMessageMutation enregistre une modification granulaire sur un message
// (Édition, Suppression, Réaction) ET signale la conversation comme modifiée aux participants.
func RecordMessageMutation(ctx context.Context, convID int64, messageID int64, participantIDs []int64) error {
	nowMs := float64(domain.NowMillis())
	msgIDStr := strconv.FormatInt(messageID, 10)

	// 1. On inscrit le message dans le Ledger (ZSET unique par conversation)
	err := redis.ConvMessageLedger.ZAdd(ctx, convID, nowMs, msgIDStr)
	if err == nil {
		_ = redis.ConvMessageLedger.RefreshTTL(ctx, convID)
	}

	// 2. On "allume le gyrophare" sur la conversation pour tous les participants
	return RecordConversationMutation(ctx, convID, participantIDs)
}

// ============================================================================
// 2. LECTURES DEPUIS LE LEDGER (PULL SYNCHRONIZATION)
// ============================================================================

// GetModifiedConversationIDs retourne la liste des IDs de conversations ayant muté depuis 'sinceMs'
func GetModifiedConversationIDs(ctx context.Context, userID int64, sinceMs int64) ([]int64, error) {
	// On cherche tous les éléments dont le score (timestamp) est strictement supérieur à sinceMs
	minScore := fmt.Sprintf("(%d", sinceMs)
	maxScore := "+inf"

	// DÉLÉGATION DDD : O(log N) abstrait et sécurisé (limité à 1000 pour la RAM)
	idsStr, err := redis.UserSyncLedger.ZRangeByScore(ctx, userID, minScore, maxScore, 1000)
	if err != nil {
		return nil, err
	}

	var convIDs []int64
	for _, idStr := range idsStr {
		if id, errParse := strconv.ParseInt(idStr, 10, 64); errParse == nil {
			convIDs = append(convIDs, id)
		}
	}
	return convIDs, nil
}

// GetModifiedMessageIDs retourne la liste des IDs de messages ayant muté dans une conversation depuis 'sinceMs'
func GetModifiedMessageIDs(ctx context.Context, convID int64, sinceMs int64) ([]int64, error) {
	minScore := fmt.Sprintf("(%d", sinceMs)
	maxScore := "+inf"

	// DÉLÉGATION DDD
	idsStr, err := redis.ConvMessageLedger.ZRangeByScore(ctx, convID, minScore, maxScore, 1000)
	if err != nil {
		return nil, err
	}

	var msgIDs []int64
	for _, idStr := range idsStr {
		if id, errParse := strconv.ParseInt(idStr, 10, 64); errParse == nil {
			msgIDs = append(msgIDs, id)
		}
	}
	return msgIDs, nil
}
