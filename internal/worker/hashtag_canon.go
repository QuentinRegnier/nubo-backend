package worker

import (
	"context"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/lib/pq"
)

// StartHashtagCanonCron lance un worker qui calcule les similarités (Levenshtein)
// entre les tags communautaires toutes les 24h pour absorber les fautes de frappe.
func StartHashtagCanonCron(ctx context.Context) {
	log.Println("🔤 Démarrage du Canoniseur de Hashtags (24h)...")
	go func() {
		// En production, utiliser un vrai cron pour viser 03:00 AM
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processHashtagCanonicalization(ctx)
			}
		}
	}()
}

func processHashtagCanonicalization(ctx context.Context) {
	// 1. Récupération de tous les tags communautaires actifs
	tags, err := redisgo.Rdb.SMembers(ctx, variables.RedisKeyActiveTagsSet).Result()
	if err != nil || len(tags) == 0 {
		return
	}

	// 1.5 PERSISTANCE SQL (Option B : Immortalisation des tags)
	persistCommunityTags(ctx, tags)

	if len(tags) < 2 {
		return // Pas assez de tags pour faire un calcul de distance
	}

	log.Printf("🔍 Canonicalisation de %d tags communautaires en cours...", len(tags))
	aliasMap := make(map[string]string)

	// Algorithme O(N^2) : Comparaison par paires.
	// (Gérable jusqu'à ~100k tags car exécuté 1 seule fois par nuit).
	for i := 0; i < len(tags); i++ {
		for j := i + 1; j < len(tags); j++ {
			t1 := tags[i]
			t2 := tags[j]

			// On ignore les tags très courts pour éviter les faux positifs (ex: "ia" et "it")
			if len(t1) < 4 || len(t2) < 4 {
				continue
			}

			// Calcul de la distance d'édition
			dist := levenshtein(t1, t2)

			// Tolérance : 1 erreur si longueur < 7, 2 erreurs si >= 7
			maxDist := 1
			if len(t1) >= 7 || len(t2) >= 7 {
				maxDist = 2
			}

			if dist <= maxDist {
				// Le plus court/ancien devient le canonique (stratégie basique)
				canon, typo := t1, t2
				if len(t2) < len(t1) {
					canon, typo = t2, t1
				}
				aliasMap[typo] = canon
			}
		}
	}

	// 2. Sauvegarde des alias dans le HASH Redis (Pipeline pour performance)
	if len(aliasMap) > 0 {
		pipe := redisgo.Rdb.Pipeline()
		for typo, canon := range aliasMap {
			pipe.HSet(ctx, variables.RedisKeyHashtagCanonMap, typo, canon)
		}
		_, err := pipe.Exec(ctx)
		if err == nil {
			log.Printf("✅ Canonicalisation terminée : %d fautes de frappes mappées.", len(aliasMap))
		}
	}
}

// levenshtein calcule la distance d'édition minimum entre deux chaînes (CPU intensive).
func levenshtein(s1, s2 string) int {
	lenS1 := len(s1)
	lenS2 := len(s2)
	row := make([]int, lenS1+1)

	for i := 0; i <= lenS1; i++ {
		row[i] = i
	}

	for i := 1; i <= lenS2; i++ {
		prev := i
		for j := 1; j <= lenS1; j++ {
			current := row[j-1]
			if s2[i-1] != s1[j-1] {
				current = min(min(row[j-1]+1, prev+1), row[j]+1)
			}
			row[j-1] = prev
			prev = current
		}
		row[lenS1] = prev
	}
	return row[lenS1]
}

// persistCommunityTags sauvegarde les tags communautaires dans PostgreSQL.
// Utilise UNNEST pour exécuter l'insertion de masse en 1 seul RTT réseau (O(1) côté Go).
func persistCommunityTags(ctx context.Context, tags []string) {
	if len(tags) == 0 {
		return
	}

	// L'instruction ON CONFLICT DO NOTHING garantit que si le tag a déjà été
	// inséré lors de la nuit précédente, PostgreSQL l'ignore sans crasher.
	query := `
		INSERT INTO content.tags (slug, is_community) 
		SELECT unnest($1::text[]), true 
		ON CONFLICT (slug) DO NOTHING
	`

	// Exécution atomique
	_, err := postgres.PostgresDB.ExecContext(ctx, query, pq.Array(tags))
	if err != nil {
		log.Printf("⚠️ Erreur lors de la persistance SQL des tags : %v", err)
	} else {
		log.Printf("💾 Persistance SQL : vérification/insertion de %d tags communautaires terminée.", len(tags))
	}
}
