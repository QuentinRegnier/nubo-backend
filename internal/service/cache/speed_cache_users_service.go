package cache

import (
	"context"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

// SearchUserByPrefix recherche des utilisateurs via l'auto-complétion (SPEED Cache)
func SearchUserByPrefix(ctx context.Context, prefix string, limit int64) ([]domain.UserLiteRequest, error) {
	// 1. Recherche ultra-rapide dans l'index lexicographique (O(log(N)))
	lexResults, err := redis.ZRangeByLex(ctx, "speed_cache:search:lex", strings.ToLower(prefix), limit)
	if err != nil {
		return nil, err
	}

	if len(lexResults) == 0 {
		return []domain.UserLiteRequest{}, nil
	}

	// 2. Extraction des IDs
	var ids []int64
	for _, res := range lexResults {
		// Le format stocké est "pseudo:id"
		parts := strings.Split(res, ":")
		if len(parts) == 2 {
			if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
	}

	// 3. Hydratation via MGET sur la collection UsersLite
	getRes, err := redis.UsersLite.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}

	var users []domain.UserLiteRequest
	// 4. On boucle sur ids pour conserver l'ordre alphabétique exact renvoyé par l'index
	for _, id := range ids {
		if data, ok := getRes.Found[id]; ok {
			var u domain.UserLiteRequest
			if err := msgpack.Unmarshal(data, &u); err == nil {
				users = append(users, u)
			}
		}
	}

	// Si un ID manque dans le cache (MissingIDs), on l'ignore silencieusement.
	// Pour de l'auto-complétion, la vitesse prime sur l'exhaustivité absolue.
	return users, nil
}
