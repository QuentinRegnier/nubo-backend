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
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : SPEED CACHE (RELATIONS ET GRAPHE SOCIAL)
// ############################################################################

// RelationValue est le moteur d'accès L1 -> L2 -> L3 pour récupérer l'état
// relationnel exact entre deux utilisateurs.
func RelationValue(ctx context.Context, targetID int64, callerID int64) int {
	callerIDString := strconv.FormatInt(callerID, 10)

	// ── ÉTAPE 1 : VÉRIFICATION RAM L1 (O(1)) ────────────────────────────────
	cachedStateString, errRedis := redis.SpeedRelations.HGet(ctx, targetID, callerIDString).Result()
	if errRedis == nil {
		if relationState, errParse := strconv.Atoi(cachedStateString); errParse == nil {
			return relationState
		}
	}

	// ── ÉTAPE 2 : COLD STORAGE L2 (MONGODB WARM STORAGE) ────────────────────
	relationStateFromMongo, errMongo := mongo.MongoGetRelationState(callerID, targetID)
	if errMongo == nil {
		_ = redis.SpeedRelations.HSet(ctx, targetID, callerIDString, relationStateFromMongo)
		return relationStateFromMongo
	}

	// ── ÉTAPE 3 : SOURCE DE VÉRITÉ L3 (POSTGRESQL) ──────────────────────────
	relationStateFromPg, errPg := postgres.FuncGetRelationState(ctx, callerID, targetID)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Int64("target_id", targetID).Int64("caller_id", callerID).Msg("Erreur L3 RelationValue")
		return variables.RelationStateNone // Zéro-valeur sécurisée par défaut
	}

	_ = redis.SpeedRelations.HSet(ctx, targetID, callerIDString, relationStateFromPg)

	// RÉHYDRATATION L2 ASYNCHRONE
	go func(currentState int) {
		backgroundCtx := context.Background()
		relationPayload := relation_models.RelationPayload{
			PrimaryID:   callerID,
			SecondaryID: targetID,
			State:       currentState,
			UpdatedAt:   domain.NowMillis(),
		}
		_ = redis.EnqueueDB(backgroundCtx, 0, targetID, redis.EntityRelation, redis.ActionUpdate, relationPayload, redis.TargetMongo)
	}(relationStateFromPg)

	return relationStateFromPg
}

// UpdateRelationState met à jour l'état de la relation (HSET) ET le tri temporel (ZSET).
func UpdateRelationState(ctx context.Context, targetID int64, callerID int64, newRelationState int, timestampMs int64) error {

	// 1. Maintien du dictionnaire d'accès O(1)
	callerIDString := strconv.FormatInt(callerID, 10)
	targetIDString := strconv.FormatInt(targetID, 10)

	errHSet := redis.SpeedRelations.HSet(ctx, targetID, callerIDString, newRelationState)
	if errHSet != nil {
		logger.Log.Error().Err(errHSet).Msg("Impossible de mettre à jour le HSET RelationValue")
		return nubo_error.NewInternal()
	}

	timestampScore := float64(timestampMs)

	// 2. Gestion des ZSETs chronologiques directionnels
	// On nettoie l'ancien état par précaution pour éviter les doublons
	_ = redis.SpeedRelationsIndex.ZRem(ctx, fmt.Sprintf("in:%d:%d", variables.RelationStateFollow, targetID), callerIDString)
	_ = redis.SpeedRelationsIndex.ZRem(ctx, fmt.Sprintf("in:%d:%d", variables.RelationStateFriend, targetID), callerIDString)
	_ = redis.SpeedRelationsIndex.ZRem(ctx, fmt.Sprintf("out:%d:%d", variables.RelationStateBlocked, callerID), targetIDString)

	// 3. Ajout dans le nouveau ZSET si l'état est actif
	if newRelationState == variables.RelationStateFollow {
		_ = redis.SpeedRelationsIndex.ZAdd(ctx, fmt.Sprintf("in:%d:%d", variables.RelationStateFollow, targetID), timestampScore, callerIDString)
	} else if newRelationState == variables.RelationStateFriend {
		_ = redis.SpeedRelationsIndex.ZAdd(ctx, fmt.Sprintf("in:%d:%d", variables.RelationStateFriend, targetID), timestampScore, callerIDString)
	} else if newRelationState == variables.RelationStateBlocked {
		_ = redis.SpeedRelationsIndex.ZAdd(ctx, fmt.Sprintf("out:%d:%d", variables.RelationStateBlocked, callerID), timestampScore, targetIDString)
	}

	return nil
}

// GetSpeedRelationsIndex récupère les abonnés en O(log N).
func GetSpeedRelationsIndex(ctx context.Context, userID int64) ([]int64, error) {
	zsetKey := fmt.Sprintf("in:%d:%d", variables.RelationStateFollow, userID)

	followerStringIDs, errRedis := redis.SpeedRelationsIndex.ZRevRange(ctx, zsetKey, 0, -1)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Msg("Impossible de lire l'index SpeedRelations des followers")
		return nil, nubo_error.NewInternal()
	}

	var followersIDsList []int64
	for _, idString := range followerStringIDs {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			followersIDsList = append(followersIDsList, parsedID)
		}
	}

	return followersIDsList, nil
}

// GetSpeedFriends récupère strictement la liste des amis.
func GetSpeedFriends(ctx context.Context, userID int64) ([]int64, error) {
	zsetKey := fmt.Sprintf("in:%d:%d", variables.RelationStateFriend, userID)

	friendStringIDs, errRedis := redis.SpeedRelationsIndex.ZRevRange(ctx, zsetKey, 0, -1)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Msg("Impossible de lire l'index SpeedRelations des amis")
		return nil, nubo_error.NewInternal()
	}

	var friendsIDsList []int64
	for _, idString := range friendStringIDs {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			friendsIDsList = append(friendsIDsList, parsedID)
		}
	}

	return friendsIDsList, nil
}

// GetFollowerCount retourne le nombre total d'abonnés en O(1).
func GetFollowerCount(ctx context.Context, userID int64) int64 {
	zsetKey := fmt.Sprintf("in:%d:%d", variables.RelationStateFollow, userID)
	count, _ := redis.SpeedRelationsIndex.ZCard(ctx, zsetKey)
	return count
}

// GetRelationsByDirectionPaginatedFromCache récupère les IDs ciblés de manière paginée en O(log N).
func GetRelationsByDirectionPaginatedFromCache(ctx context.Context, primaryID int64, relationState int, searchDirection string, fetchLimit int, fetchOffset int) ([]int64, error) {
	var targetZsetKey string

	// Format imposé : in:{state}:{targetID} ou out:{state}:{callerID}
	if searchDirection == "incoming" {
		targetZsetKey = fmt.Sprintf("in:%d:%d", relationState, primaryID)
	} else {
		targetZsetKey = fmt.Sprintf("out:%d:%d", relationState, primaryID)
	}

	targetStringsIDs, errRedis := redis.SpeedRelationsIndex.ZRevRange(ctx, targetZsetKey, int64(fetchOffset), int64(fetchOffset+fetchLimit-1))
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Msg("Erreur lors de la récupération paginée du SpeedRelationsIndex")
		return nil, nubo_error.NewInternal()
	}

	var matchedIDsList []int64
	for _, idString := range targetStringsIDs {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			matchedIDsList = append(matchedIDsList, parsedID)
		}
	}

	return matchedIDsList, nil
}
