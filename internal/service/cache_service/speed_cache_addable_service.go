package cache_service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

// GetAddableUsersFromSpeedCache résout le sous-cache ZSET (Ami > Abonné + Lexicographique) en O(log N)
func GetAddableUsersFromSpeedCache(ctx context.Context, callerID int64, limit int64, offset int64, force bool) ([]lite_models.UserLiteRequest, error) {
	// Utilisation propre de la collection déclarée dans le repository (manager.go)
	zsetKey := redis.SpeedAddable.Key(callerID)

	// 1. Mode Force : Purge manuelle via la primitive d'abstraction
	if force {
		_ = redis.Del(ctx, zsetKey)
	}

	// Utilisation de l'abstraction Exists (retourne directement un booléen)
	exists, err := redis.Exists(ctx, zsetKey)
	if err != nil {
		return nil, err
	}

	// 2. CACHE MISS : Reconstruction intelligente
	if !exists {
		relations, errPg := postgres.FuncLoadAddableRelations(ctx, callerID)
		if errPg != nil {
			return nil, errPg
		}

		if len(relations) > 0 {
			var targetIDs []int64
			stateMap := make(map[int64]int)
			for _, r := range relations {
				targetIDs = append(targetIDs, r.TargetID)
				stateMap[r.TargetID] = r.State
			}

			// Récupération des données compressées L1 (MGET natif du manager)
			getRes, _ := redis.UsersLite.GetMany(ctx, targetIDs)

			for _, targetID := range targetIDs {
				data, ok := getRes.Found[targetID]
				if !ok {
					continue
				}

				var u lite_models.UserLiteRequest
				if err := msgpack.Unmarshal(data, &u); err == nil {
					// ASTUCE DE TRI :
					// State 2 (Ami) devient 0. State 1 (Abonné) devient 1.
					// Résultat dans le ZSET : "0_alice_123" arrivera toujours avant "1_bob_456" !
					invertedState := 2 - stateMap[targetID]
					member := fmt.Sprintf("%d_%s_%d", invertedState, strings.ToLower(u.Username), u.ID)

					// Appel pur au repository/redis (Zéro appel à go-redis / redisgo.Z)
					_ = redis.ZAdd(ctx, zsetKey, 0, member)
				}
			}

			// TTL Glissant : C'est un sous-cache volatile via abstraction
			_ = redis.Expire(ctx, zsetKey, 5*time.Minute)

		} else {
			// Marqueur de vide pour éviter le martèlement BDD (Anti-Cache-Hole)
			_ = redis.ZAdd(ctx, zsetKey, 0, "-1")
			_ = redis.Expire(ctx, zsetKey, 5*time.Minute)
		}
	}

	// 3. LECTURE PAGINÉE (O(log(N))) via abstraction
	members, err := redis.ZRange(ctx, zsetKey, offset, offset+limit-1)
	if err != nil {
		return nil, err
	}

	var finalIDs []int64
	for _, m := range members {
		if m == "-1" {
			continue // Marqueur de vide
		}
		parts := strings.Split(m, "_")
		if len(parts) == 3 {
			if id, errParse := strconv.ParseInt(parts[2], 10, 64); errParse == nil {
				finalIDs = append(finalIDs, id)
			}
		}
	}

	if len(finalIDs) == 0 {
		return []lite_models.UserLiteRequest{}, nil
	}

	// 4. HYDRATATION FINALE (MGET O(1) sur le Speed Cache)
	getRes, err := redis.UsersLite.GetMany(ctx, finalIDs)
	if err != nil {
		return nil, err
	}

	var result []lite_models.UserLiteRequest
	for _, id := range finalIDs {
		if data, ok := getRes.Found[id]; ok {
			var u lite_models.UserLiteRequest
			if err := msgpack.Unmarshal(data, &u); err == nil {
				result = append(result, u)
			}
		}
	}

	return result, nil
}
