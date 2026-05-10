package service

import (
	"context"
	"strconv"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// PIPELINE DE MISE À JOUR REDIS — TDD §3.3 / §3.4
// ============================================================================

// luaZAddWithCap est le script Lua exécuté atomiquement côté Redis.
//
// TDD §3.3 — Mécanisme de plafonnement à 500 éléments par ZSET:
//
//	Garanties:
//	- Insertion + éjection dans la même transaction atomique (pas de race condition)
//	- Seuls les 500 posts au score le plus élevé sont conservés
//	- Complexité: O(log N) pour ZADD + O(log N + K) pour ZREMRANGEBYRANK
const luaZAddWithCap = `
local key      = KEYS[1]
local score    = tonumber(ARGV[1])
local member   = ARGV[2]
local max_size = tonumber(ARGV[3])

redis.call('ZADD', key, score, member)

local current_size = redis.call('ZCARD', key)
if current_size > max_size then
    redis.call('ZREMRANGEBYRANK', key, 0, current_size - max_size - 1)
end

return redis.call('ZSCORE', key, member)
`

// ZAddWithCap insère atomiquement un post dans un ZSET Redis plafonné à MAX_ZSET.
//
// TDD §3.3 / §3.4 — Pipeline de Mise à Jour Redis:
//
//	ZADD trend:global:hourly:{HH} score post_id
//	→ plafonnement atomique à MAX_ZSET=500 via script Lua
func ZAddWithCap(ctx context.Context, key string, score float64, postID int64) error {
	return redisgo.Rdb.Eval(
		ctx,
		luaZAddWithCap,
		[]string{key},
		score,
		strconv.FormatInt(postID, 10),
		variables.TDDMaxZSET,
	).Err()
}

// ComputeHashtagTrendScore calcule T(h, t) — le score de tendance d'un hashtag canonique.
//
// TDD §3.3 — Formule:
//
//	T(h, t) = Σ_{p ∈ P_h} S(p,t) · I[Δt(p) ≤ 48·3600]
//
// Paramètres:
//   - postScores: map[postID → S(p,t)] pour les posts contenant le hashtag h
//   - postAges: map[postID → Δt(p) en secondes]
func ComputeHashtagTrendScore(postScores map[int64]float64, postAges map[int64]float64) float64 {
	// TDD §3.3: fenêtre temporelle de 48 heures
	const maxAgeSecs = 48.0 * 3600.0

	var total float64
	for postID, score := range postScores {
		// I[Δt(p) ≤ 48·3600] — indicateur temporel
		if age, ok := postAges[postID]; ok && age <= maxAgeSecs {
			total += score
		}
	}
	return total
}

// UpdateTrendZSETs met à jour les ZSETs de tendance Redis pour un post et ses hashtags.
//
// TDD §3.4 — Pipeline Write-Behind:
//
//	[Événement engagement] → [Redis Queue] → [Worker Go] → [ZADD trend:*]
//
// La fréquence de recalcul est tiered selon l'âge du post:
//   - < 6h: toutes les 2 min
//   - 6–24h: toutes les 15 min
//   - 24–72h: toutes les 60 min
//   - > 72h: toutes les 6h
funcUpdateTrendZSETs(ctx context.Context, postID int64, score float64, hashtags []string, date, hour string) error {
	// ZADD + plafonnement atomique pour le bucket horaire
	// TDD §3.4: trend:global:hourly:{YYYYMMDDHH}
	hourlyKey := fmt.Sprintf(variables.RedisKeyTrendGlobalHourly, hour)
	if err := ZAddWithCap(ctx, hourlyKey, score, postID); err != nil {
		return fmt.Errorf("zadd hourly zset: %w", err)
	}

	// ZADD + plafonnement atomique pour le bucket quotidien
	// TDD §3.4: trend:global:daily:{YYYYMMDD}
	dailyKey := fmt.Sprintf(variables.RedisKeyTrendGlobalDaily, date)
	if err := ZAddWithCap(ctx, dailyKey, score, postID); err != nil {
		return fmt.Errorf("zadd daily zset: %w", err)
	}

	// ZADD pour chaque hashtag canonique
	// TDD §3.4: trend:tag:{tag_canonique}:daily
	for _, tag := range hashtags {
		canonical := NormalizeHashtag(tag)
		if canonical == "" {
			continue
		}
		tagKey := fmt.Sprintf(variables.RedisKeyTrendTagDaily, canonical)
		if err := ZAddWithCap(ctx, tagKey, score, postID); err != nil {
			return fmt.Errorf("zadd tag zset [%s]: %w", canonical, err)
		}
	}

	return nil
}
