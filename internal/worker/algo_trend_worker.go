package worker

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// StartHashtagTrendCron lance l'évaluation des tendances mondiales de hashtags (TDD §3.3).
// Il tourne toutes les 15 minutes pour maintenir le Top 100 des tags sans saturer le CPU.
func StartHashtagTrendCron(ctx context.Context) {
	log.Println("📈 Démarrage du Moteur de Tendances Hashtags (15m)...")
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processHashtagTrends(ctx)
			}
		}
	}()
}

func processHashtagTrends(ctx context.Context) {
	// 1. Extraction des meilleurs posts mondiaux (Most Cache L1)
	now := time.Now().UTC()
	dateStr := now.Format("20060102") // Format attendu par UpdateTrendZSETs

	// Reconstruction de la clé physique du ZSET telle que définie dans le TDD / most_cache_service
	dailyGlobalKey := fmt.Sprintf(variables.RedisKeyTrendGlobalDaily, dateStr)

	// Utilisation du client Redis brut (Rdb) en attendant la refonte des Collections
	topPosts, err := redis.ZRevRangeWithScores(ctx, dailyGlobalKey, 0, 1000)
	if err != nil || len(topPosts) == 0 {
		return
	}

	postScoresByTag := make(map[string]map[int64]float64)
	postAgesByTag := make(map[string]map[int64]float64)

	// 2. Hydratation et Groupement O(N)
	for _, item := range topPosts {
		var postID int64
		var errParse error

		// go-redis renvoie Member sous forme d'interface{}.
		// Selon la façon dont il a été inséré, ça peut être un string ou autre.
		switch v := item.Member.(type) {
		case string:
			postID, errParse = strconv.ParseInt(v, 10, 64)
		case []byte:
			postID, errParse = strconv.ParseInt(string(v), 10, 64)
		default:
			// Si c'est un format inattendu, on force la conversion string
			postID, errParse = strconv.ParseInt(fmt.Sprintf("%v", v), 10, 64)
		}

		if errParse != nil || postID == 0 {
			continue
		}

		score := item.Score

		// Hydratation via le Cache L1 (ou L2/L3 en fallback)
		// (getPostWithFallback est déjà défini dans most_cache_worker.go)
		p, err := getPostWithFallback(ctx, postID)
		if err != nil || p.Visibility != 0 {
			continue // Sécurité : On ignore les posts privés pour les tendances mondiales
		}

		ageSeconds := now.Sub(p.CreatedAt).Seconds()

		// Agrégation par Tag
		for _, tag := range p.Hashtags {
			if postScoresByTag[tag] == nil {
				postScoresByTag[tag] = make(map[int64]float64)
				postAgesByTag[tag] = make(map[int64]float64)
			}
			postScoresByTag[tag][postID] = score
			postAgesByTag[tag][postID] = ageSeconds
		}
	}

	// 3. Calcul mathématique de T(h,t) et Persistance via Pipeline
	if len(postScoresByTag) > 0 {
		pipe := redis.Tags.Pipeline()
		trendingKey := redis.Tags.Key("trending")

		for tag, scoresMap := range postScoresByTag {
			agesMap := postAgesByTag[tag]

			// Appel du moteur mathématique pur (TDD §3.3)
			trendScore := service.ComputeHashtagTrendScore(scoresMap, agesMap)

			if trendScore > 0 {
				pipe.Do(ctx, "ZADD", trendingKey, trendScore, tag)
			}
		}

		// Protection RAM (LFU/Cap) : On ne conserve que le Top 100 des tendances
		pipe.Do(ctx, "ZREMRANGEBYRANK", trendingKey, 0, -101)

		_, err = pipe.Exec(ctx)
		if err != nil {
			log.Printf("❌ [Hashtag Trends] Échec de la sauvegarde L1 : %v", err)
		} else {
			log.Printf("✅ [Hashtag Trends] Mise à jour du Top 100 réussie.")
		}
	}
}
