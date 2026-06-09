package cache_service

import (
	"context"
	"fmt"
	"log"
	"strconv"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
)

const (
	// RedisKeySpeedFollowers maintient la LISTE des abonnés pour le Fan-Out (Worker)
	RedisKeySpeedFollowers = "speed:followers:%d"
	// RedisKeySpeedRelations maintient l'ÉTAT exact (0, 1, 2, -1) pour les droits d'accès
	RedisKeySpeedRelations = "speed:relations:%d"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1. LE MOTEUR D'ACCÈS L1 -> L2 -> L3 (La nouveauté)
// ─────────────────────────────────────────────────────────────────────────────

// RelationValue retourne l'état strict de la relation (0 = Rien, 1 = Follow, 2 = Ami, -1 = Banni).
// Fonctionne en cascade : RAM (Redis) -> Cold (Mongo) -> Source (Postgres).
func RelationValue(ctx context.Context, targetID int64, callerID int64) int {
	// Clé du Hash Redis : speed:relations:<targetID>
	// Champ du Hash : <callerID>
	key := fmt.Sprintf(RedisKeySpeedRelations, targetID)
	strCallerID := strconv.FormatInt(callerID, 10)

	// Étape 1 : Cache L1 (Speed Cache en RAM pure, ~0.1ms)
	val, err := redisgo.Rdb.HGet(ctx, key, strCallerID).Result()
	if err == nil {
		if state, errConv := strconv.Atoi(val); errConv == nil {
			return state
		}
	}

	// Étape 2 : Cold Storage L2 (MongoDB, ~5ms)
	state, errMongo := mongo.MongoGetRelationState(callerID, targetID)
	if errMongo == nil {
		// Réhydratation L1
		_ = redisgo.Rdb.HSet(ctx, key, strCallerID, state).Err()
		return state
	}

	// Étape 3 : Source of Truth L3 (PostgreSQL, ~10ms)
	statePg, errPg := postgres.FuncGetRelationState(ctx, callerID, targetID)
	if errPg != nil {
		log.Printf("⚠️ Erreur L3 RelationValue (Target: %d, Caller: %d): %v", targetID, callerID, errPg)
		return 0 // Par sécurité absolue, on refuse l'accès en cas de crash BDD
	}

	// Réhydratation L1 (Inclut le Cache Négatif : si statePg == 0, on le met en RAM quand même)
	_ = redisgo.Rdb.HSet(ctx, key, strCallerID, statePg).Err()

	return statePg
}

// ─────────────────────────────────────────────────────────────────────────────
// 2. GESTION DU FAN-OUT ET MISES À JOUR (Rétrocompatibilité)
// ─────────────────────────────────────────────────────────────────────────────

// UpdateRelationState met à jour l'état de la relation (à appeler depuis les handlers de follow/unfollow/ban)
func UpdateRelationState(ctx context.Context, targetID int64, callerID int64, newState int) error {
	// 1. Met à jour le dictionnaire d'accès
	keyRelations := fmt.Sprintf(RedisKeySpeedRelations, targetID)
	err := redisgo.Rdb.HSet(ctx, keyRelations, strconv.FormatInt(callerID, 10), newState).Err()

	// 2. Maintien de l'ancien Set pour le worker Fan-Out
	keyFollowers := fmt.Sprintf(RedisKeySpeedFollowers, targetID)
	if newState == 1 || newState == 2 {
		_ = redisgo.Rdb.SAdd(ctx, keyFollowers, callerID).Err()
	} else {
		_ = redisgo.Rdb.SRem(ctx, keyFollowers, callerID).Err()
	}

	return err
}

// GetSpeedFollowers récupère les abonnés pour le Fan-Out asynchrone (Worker).
func GetSpeedFollowers(ctx context.Context, userID int64) ([]int64, error) {
	key := fmt.Sprintf(RedisKeySpeedFollowers, userID)

	followerStrings, err := redisgo.Rdb.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("erreur lecture speed cache followers: %w", err)
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
	key := fmt.Sprintf(RedisKeySpeedFollowers, userID)
	count, _ := redisgo.Rdb.SCard(ctx, key).Result()
	return count
}

// GetSpeedFriends récupère strictement la liste des amis (Relation = 2).
// Utilisé pour le Fan-Out restreint des posts privés.
func GetSpeedFriends(ctx context.Context, userID int64) ([]int64, error) {
	key := fmt.Sprintf(RedisKeySpeedRelations, userID)

	relations, err := redisgo.Rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("erreur lecture speed cache relations: %w", err)
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
