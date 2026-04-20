package redis

import (
	"context"
	"fmt"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/go-redis/redis/v8"
)

// ============================================================================
// PRIMITIVES ZSET (SORTED SETS) - Pour le MOST Cache
// ============================================================================

// ZAdd ajoute un élément avec un score (ou met à jour son score).
// Complexité : O(log(N))
// Utilisé pour :
// - Ajouter un Post dans un Tag (Score = Timestamp)
// - Ajouter un Post sur le Profil User (Score = Timestamp)
func ZAdd(ctx context.Context, key string, score float64, member interface{}) error {
	return redisgo.Rdb.ZAdd(ctx, key, &redis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

// ZIncrBy incrémente le score d'un membre existant.
// Complexité : O(log(N))
// Utilisé pour :
// - Augmenter le compteur de Vues ou Likes (Score = Compteur)
func ZIncrBy(ctx context.Context, key string, increment float64, member interface{}) error {
	// On convertit le member en string de manière sécurisée (gère les int64, string, etc.)
	memberStr := fmt.Sprintf("%v", member)
	return redisgo.Rdb.ZIncrBy(ctx, key, increment, memberStr).Err()
}

// ZRevRange récupère une liste d'éléments triés du plus grand score au plus petit.
// Complexité : O(log(N) + M)
// Utilisé pour :
// - Récupérer les posts les plus récents (Timeline)
// - Récupérer les posts les plus populaires (Top Trending)
// Retourne : []string (les IDs des posts)
func ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return redisgo.Rdb.ZRevRange(ctx, key, start, stop).Result()
}

// ZRemRangeByRank supprime les éléments selon leur position (rang) dans le tri.
// Complexité : O(log(N) + M)
// C'est la fonction CLÉ pour le "Capping" (Nettoyage automatique).
//
// NOTE : Dans Redis, le rang 0 est le plus petit score.
// Pour ne garder que les 5000 meilleurs (les plus gros scores),
// il faut supprimer du rang 0 jusqu'au rang -5001.
func ZRemRangeByRank(ctx context.Context, key string, start, stop int64) error {
	return redisgo.Rdb.ZRemRangeByRank(ctx, key, start, stop).Err()
}

// ZScore récupère le score actuel d'un membre.
// Utile pour vérifier si un post est déjà classé ou connaître son nombre de vues exact.
func ZScore(ctx context.Context, key string, member interface{}) (float64, error) {
	// fmt.Sprintf("%v") permet de gérer int64 ou string de façon transparente
	return redisgo.Rdb.ZScore(ctx, key, fmt.Sprintf("%v", member)).Result()
}

// ZCount compte le nombre d'éléments entre min et max score.
func ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return redisgo.Rdb.ZCount(ctx, key, min, max).Result()
}

// ZCard donne la taille totale du set (nombre d'éléments).
func ZCard(ctx context.Context, key string) (int64, error) {
	return redisgo.Rdb.ZCard(ctx, key).Result()
}
