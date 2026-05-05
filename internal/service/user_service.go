package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/vmihailenco/msgpack/v5"
)

// AddPostToUserProfile ajoute un post au ZSET de l'utilisateur avec un score chronologique précis.
// Le score est le timestamp Unix (ms) de création du post.
func AddPostToUserProfile(ctx context.Context, userID int64, postID int64, createdAt int64) {
	key := fmt.Sprintf("user:posts:%d", userID)
	score := float64(createdAt)

	// Ajouter au ZSET via ton wrapper de repository
	if err := redis.ZAdd(ctx, key, score, postID); err != nil {
		return
	}

	// Éviction : on ne garde que les X derniers
	_ = redis.ZRemRangeByRank(ctx, key, 0, -(variables.MaxUserPostsElements + 1))
}

// InvalidateUserProfileCache détruit le ZSET d'un utilisateur pour forcer une réhydratation
// complète depuis PostgreSQL au prochain appel (Cache Busting).
func InvalidateUserProfileCache(ctx context.Context, userID int64) {
	key := fmt.Sprintf("user:posts:%d", userID)

	// Utilisation directe du client de l'infrastructure pour un DEL massif en O(1)
	_ = redisgo.Rdb.Del(ctx, key).Err()
}

// SearchUserByPrefix recherche des utilisateurs via l'auto-complétion (SPEED Cache)
func SearchUserByPrefix(ctx context.Context, prefix string, limit int64) ([]domain.UserLiteRequest, error) {
	// 1. Recherche ultra-rapide dans l'index lexicographique (O(log(N)))
	lexResults, err := redis.ZRangeByLex(ctx, "users:search:lex", strings.ToLower(prefix), limit)
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
