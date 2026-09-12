package cache_service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

// StoreCommunityLiteInSpeedCache sauvegarde directement un objet CommunityLiteRequest et met à jour l'index Lexicographique
// (Prêt pour être utilisé par les workers à l'avenir)
func StoreCommunityLiteInSpeedCache(ctx context.Context, lite lite_models.CommunityLiteRequest) error {
	lexValue := fmt.Sprintf("%s:%d", strings.ToLower(lite.Name), lite.ID)
	_ = redis.CommunitiesLex.ZAdd(ctx, "lex", 0, lexValue)
	return redis.SpeedCommunity.SetObject(ctx, lite.ID, lite)
}

// SearchCommunitiesByPrefix recherche des communautés en O(log(N)) RAM et les réhydrate
func SearchCommunitiesByPrefix(ctx context.Context, prefix string, limit int64) ([]lite_models.CommunityLiteRequest, error) {
	// 1. Recherche ultra-rapide dans l'index lexicographique
	lexResults, err := redis.CommunitiesLex.ZRangeByLex(ctx, "lex", strings.ToLower(prefix), limit)
	if err != nil {
		return nil, err
	}

	if len(lexResults) == 0 {
		return []lite_models.CommunityLiteRequest{}, nil
	}

	// 2. Extraction des IDs
	var ids []int64
	for _, res := range lexResults {
		// Le format stocké est "nom:id"
		parts := strings.Split(res, ":")
		if len(parts) == 2 {
			if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
	}

	// 3. Hydratation via MGET sur la collection SpeedCommunity
	getRes, err := redis.SpeedCommunity.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}

	var communities []lite_models.CommunityLiteRequest
	// 4. On boucle sur ids pour conserver l'ordre alphabétique exact renvoyé par l'index
	for _, id := range ids {
		if data, ok := getRes.Found[id]; ok {
			var c lite_models.CommunityLiteRequest
			if err := msgpack.Unmarshal(data, &c); err == nil {
				communities = append(communities, c)
			}
		}
	}

	return communities, nil
}
