package cache_service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1. LE MOTEUR D'ACCÈS L1 -> L2 -> L3 (La nouveauté)
// ─────────────────────────────────────────────────────────────────────────────

// RelationValue retourne l'état strict de la relation (0 = Rien, 1 = Follow, 2 = Ami, -1 = Banni).
// Fonctionne en cascade : RAM (Redis) -> Cold (Mongo) -> Source (Postgres).
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
		// Réhydratation L1
		_ = redis.SpeedRelations.HSet(ctx, targetID, strCallerID, state)
		return state
	}

	// Étape 3 : Source of Truth L3 (PostgreSQL, ~10ms)
	statePg, errPg := postgres.FuncGetRelationState(ctx, callerID, targetID)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Int64("target_id", targetID).Int64("caller_id", callerID).Msg("Erreur L3 RelationValue")
		return 0 // Par sécurité absolue, on refuse l'accès en cas de crash BDD
	}

	// Réhydratation L1 (Inclut le Cache Négatif)
	_ = redis.SpeedRelations.HSet(ctx, targetID, strCallerID, statePg)

	// Réhydratation L2 asynchrone via la queue
	go func(currentState int) {
		bgCtx := context.Background()
		payload := relation_models.RelationPayload{
			PrimaryID:   callerID,
			SecondaryID: targetID,
			State:       currentState,
			UpdatedAt:   time.Now().UTC(),
		}
		// On envoie un ActionUpdate. Le worker Mongo a été codé pour utiliser PrimaryID/SecondaryID
		_ = redis.EnqueueDB(bgCtx, 0, targetID, redis.EntityRelation, redis.ActionUpdate, payload, redis.TargetMongo)
	}(statePg)

	return statePg
}

// ─────────────────────────────────────────────────────────────────────────────
// 2. GESTION DU FAN-OUT ET MISES À JOUR (Rétrocompatibilité)
// ─────────────────────────────────────────────────────────────────────────────

// UpdateRelationState met à jour l'état de la relation (à appeler depuis les handlers de follow/unfollow/ban)
func UpdateRelationState(ctx context.Context, targetID int64, callerID int64, newState int) error {
	// 1. Met à jour le dictionnaire d'accès
	err := redis.SpeedRelations.HSet(ctx, targetID, strconv.FormatInt(callerID, 10), newState)

	// 2. Maintien de l'ancien Set pour le worker Fan-Out
	if newState == 1 || newState == 2 {
		_ = redis.SpeedFollowers.SAdd(ctx, targetID, callerID)
	} else {
		_ = redis.SpeedFollowers.SRem(ctx, targetID, callerID)
	}

	return err
}

// GetSpeedFollowers récupère les abonnés pour le Fan-Out asynchrone (Worker).
func GetSpeedFollowers(ctx context.Context, userID int64) ([]int64, error) {
	followerStrings, err := redis.SpeedFollowers.SMembers(ctx, userID)
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

// GetFollowerCount retourne le nombre total d'abonnés d'un utilisateur en O(1).
// Idéal pour la protection "Anti-Crash Justin Bieber" avant un Fan-Out.
func GetFollowerCount(ctx context.Context, userID int64) int64 {
	count, _ := redis.SpeedFollowers.SCard(ctx, userID)
	return count
}

// GetSpeedFriends récupère strictement la liste des amis (Relation = 2).
// Utilisé pour le Fan-Out restreint des posts privés.
func GetSpeedFriends(ctx context.Context, userID int64) ([]int64, error) {
	relations, err := redis.SpeedRelations.HGetAll(ctx, userID).Result()
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}

	var friends []int64
	for callerIDStr, stateStr := range relations {
		if stateStr == "2" { // 2 = État "Ami" strict
			if callerID, err := strconv.ParseInt(callerIDStr, 10, 64); err == nil {
				friends = append(friends, callerID)
			}
		}
	}

	return friends, nil
}
