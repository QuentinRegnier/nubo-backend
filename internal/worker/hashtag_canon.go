package worker

import (
	"context"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
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
	// 1. Récupération de tous les tags communautaires actifs via Collection
	tags, err := redis.Tags.SMembers(ctx, "active")
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

			// Utilisation du Levenshtein normalisé de Claude (Gère le Français et les Runes)
			distNorm := NormalizedLevenshtein(t1, t2)

			// Critère TDD : Distance <= 0.15 ET même racine morphologique
			if distNorm <= 0.15 && service.StemHashtag(t1) == service.StemHashtag(t2) {
				canon, typo := t1, t2
				if len(t2) < len(t1) {
					canon, typo = t2, t1
				}
				aliasMap[typo] = canon
			}
		}
	}

	// 2. Sauvegarde des alias dans le HASH Redis (Pipeline pour performance via Collection)
	if len(aliasMap) > 0 {
		pipe := redis.HashtagCanon.Pipeline()
		for typo, canon := range aliasMap {
			// On demande proprement la clé finale à la Collection
			pipe.HSet(ctx, redis.HashtagCanon.Key("map"), typo, canon)
		}
		_, err := pipe.Exec(ctx)
		if err == nil {
			log.Printf("✅ Canonicalisation terminée : %d fautes de frappes mappées.", len(aliasMap))
		}
	}
}

// NormalizedLevenshtein calcule la distance de Levenshtein normalisée.
//
// TDD §3.3 — Formule:
//
//	d_Lev(h_i, h_j) = Lev(h_i, h_j) / max(|h_i|, |h_j|)
//
// Retourne une valeur dans [0.0, 1.0]:
//
//	0.0 = chaînes identiques,  1.0 = chaînes totalement différentes.
func NormalizedLevenshtein(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0.0
	}
	return float64(levenshteinRunes(ra, rb)) / float64(maxLen)
}

// levenshteinRunes calcule la distance d'édition de Levenshtein entre deux slices de runes.
// Implémentation optimisée en espace O(min(|a|,|b|)) via deux rangées glissantes.
func levenshteinRunes(a, b []rune) int {
	la, lb := len(a), len(b)
	// Optimisation: garantir |a| ≤ |b| pour minimiser l'allocation mémoire.
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}

	// Deux rangées de la matrice DP (espace O(la) au lieu de O(la·lb)).
	prev := make([]int, la+1)
	curr := make([]int, la+1)

	// Initialisation: prev[j] = j (coût de suppression des j premiers chars de a).
	for j := 0; j <= la; j++ {
		prev[j] = j
	}

	for i := 1; i <= lb; i++ {
		curr[0] = i
		for j := 1; j <= la; j++ {
			cost := 1
			if b[i-1] == a[j-1] {
				cost = 0
			}
			// min(insertion, suppression, substitution)
			curr[j] = min3Int(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[la]
}

func min3Int(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
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
