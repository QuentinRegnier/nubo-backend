package cache_service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// 1. LE MOTEUR D'ACCÈS L1 -> L2 -> L3
func RelationValue(ctx context.Context, targetID int64, callerID int64) int {
	strCallerID := strconv.FormatInt(callerID, 10)

	// Étape 1 : Vérification rapide en RAM (Speed Cache L1)
	val, err := redis.SpeedRelations.HGet(ctx, targetID, strCallerID).Result()
	if err == nil {
		if state, errConv := strconv.Atoi(val); errConv == nil {
			return state
		}
	}

	// Étape 2 : Cold Storage L2 (MongoDB, ~5ms)
	state, errMongo := mongo.MongoGetRelationState(callerID, targetID)
	if errMongo == nil {
		_ = redis.SpeedRelations.HSet(ctx, targetID, strCallerID, state)
		return state
	}

	// Étape 3 : Source of Truth L3 (PostgreSQL, ~10ms)
	statePg, errPg := postgres.FuncGetRelationState(ctx, callerID, targetID)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Int64("target_id", targetID).Int64("caller_id", callerID).Msg("Erreur L3 RelationValue")
		return 0
	}

	_ = redis.SpeedRelations.HSet(ctx, targetID, strCallerID, statePg)

	// Réhydratation L2 asynchrone via la queue
	go func(currentState int) {
		bgCtx := context.Background()
		payload := relation_models.RelationPayload{
			PrimaryID:   callerID,
			SecondaryID: targetID,
			State:       currentState,
			UpdatedAt:   domain.NowMillis(),
		}
		_ = redis.EnqueueDB(bgCtx, 0, targetID, redis.EntityRelation, redis.ActionUpdate, payload, redis.TargetMongo)
	}(statePg)

	return statePg
}

// UpdateRelationState met à jour l'état de la relation ET le tri temporel
// ✅ NOUVEAU : Ajout du paramètre timestampMs pour le ZSET
func UpdateRelationState(ctx context.Context, targetID int64, callerID int64, newState int, timestampMs int64) error {
	// 1. Maintien du dictionnaire d'accès O(1)
	err := redis.SpeedRelations.HSet(ctx, targetID, strconv.FormatInt(callerID, 10), newState)

	strCaller := strconv.FormatInt(callerID, 10)
	strTarget := strconv.FormatInt(targetID, 10)
	score := float64(timestampMs)

	// 2. Gestion des ZSETs chronologiques
	// (On utilise SpeedRelationsIndex pour tout, en séparant les clés par direction et état)
	// On nettoie l'ancien état par précaution pour éviter les doublons directionnels
	_ = redis.SpeedRelationsIndex.ZRem(ctx, fmt.Sprintf("in:1:%d", targetID), strCaller)
	_ = redis.SpeedRelationsIndex.ZRem(ctx, fmt.Sprintf("in:2:%d", targetID), strCaller)
	_ = redis.SpeedRelationsIndex.ZRem(ctx, fmt.Sprintf("out:-1:%d", callerID), strTarget)

	// 3. Ajout dans le nouveau ZSET si l'état est actif
	if newState == 1 { // Follower (Incoming pour targetID)
		_ = redis.SpeedRelationsIndex.ZAdd(ctx, fmt.Sprintf("in:1:%d", targetID), score, strCaller)
	} else if newState == 2 { // Ami (Incoming pour targetID)
		_ = redis.SpeedRelationsIndex.ZAdd(ctx, fmt.Sprintf("in:2:%d", targetID), score, strCaller)
	} else if newState == -1 { // Bloqué (Outgoing pour callerID)
		_ = redis.SpeedRelationsIndex.ZAdd(ctx, fmt.Sprintf("out:-1:%d", callerID), score, strTarget)
	}

	return err
}

// GetSpeedRelationsIndex récupère les abonnés en O(log N)
func GetSpeedRelationsIndex(ctx context.Context, userID int64) ([]int64, error) {
	followerStrings, err := redis.SpeedRelationsIndex.ZRevRange(ctx, fmt.Sprintf("in:1:%d", userID), 0, -1)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	var followers []int64
	for _, idStr := range followerStrings {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			followers = append(followers, id)
		}
	}
	return followers, nil
}

// GetSpeedFriends récupère strictement la liste des amis
func GetSpeedFriends(ctx context.Context, userID int64) ([]int64, error) {
	friendStrings, err := redis.SpeedRelationsIndex.ZRevRange(ctx, fmt.Sprintf("in:2:%d", userID), 0, -1)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	var friends []int64
	for _, idStr := range friendStrings {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			friends = append(friends, id)
		}
	}
	return friends, nil
}

// GetFollowerCount retourne le nombre total d'abonnés en O(1)
func GetFollowerCount(ctx context.Context, userID int64) int64 {
	count, _ := redis.SpeedRelationsIndex.ZCard(ctx, fmt.Sprintf("in:1:%d", userID))
	return count
}

// GetRelationsByDirectionPaginatedFromCache récupère les IDs ciblés de manière paginée en O(log N) depuis le ZSET (L1)
func GetRelationsByDirectionPaginatedFromCache(ctx context.Context, primaryID int64, state int, direction string, limit int, offset int) ([]int64, error) {
	var zsetKey string
	// Format imposé par UpdateRelationState: in:{state}:{targetID} ou out:{state}:{callerID}
	if direction == "incoming" {
		zsetKey = fmt.Sprintf("in:%d:%d", state, primaryID)
	} else {
		zsetKey = fmt.Sprintf("out:%d:%d", state, primaryID)
	}

	// On interroge SpeedRelationsIndex (qui est notre index de graphe social trié)
	targetIDsStr, err := redis.SpeedRelationsIndex.ZRevRange(ctx, zsetKey, int64(offset), int64(offset+limit-1))
	if err != nil {
		return nil, err
	}

	var matchedIDs []int64
	for _, idStr := range targetIDsStr {
		if id, errParse := strconv.ParseInt(idStr, 10, 64); errParse == nil {
			matchedIDs = append(matchedIDs, id)
		}
	}

	return matchedIDs, nil
}
