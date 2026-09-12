package search_service

import (
	"context"
	"sort"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// AutocompleteTags orchestre la recherche ultra-rapide de hashtags et les suggestions sémantiques.
func AutocompleteTags(ctx context.Context, input search_models.AutocompleteTagInput) (search_models.AutocompleteTagOutput, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 15 // Limite intelligente par défaut
	}

	query := strings.ToLower(strings.TrimSpace(input.Query))
	var finalTags []string
	seen := make(map[string]bool)

	// 1. RECHERCHE LEXICOGRAPHIQUE (Autocomplétion classique en O(log N))
	lexTags, err := cache_service.SearchTagsByPrefix(ctx, query, limit)
	if err != nil {
		return search_models.AutocompleteTagOutput{}, err
	}

	// On vérifie si la requête de l'utilisateur correspond exactement à un tag existant
	isExactMatch := false
	for _, t := range lexTags {
		if t == query {
			isExactMatch = true
			break
		}
	}

	// 2. MAGIE SÉMANTIQUE : Si match exact, on injecte le nuage de tags (Le Graphe de Markov)
	if isExactMatch {
		// On force le tag exact en première position absolue
		finalTags = append(finalTags, query)
		seen[query] = true

		// Interrogation du Cache Sémantique (O(1) en RAM)
		relatedMap := cache_service.GetRelatedTagsLazy(ctx, query)
		if len(relatedMap) > 0 {
			// Création d'une structure temporaire pour trier par poids (pertinence algorithmique)
			type tagWeight struct {
				Tag    string
				Weight float64
			}
			var related []tagWeight
			for t, w := range relatedMap {
				related = append(related, tagWeight{Tag: t, Weight: w})
			}

			// Tri décroissant sur la force du lien sémantique
			sort.Slice(related, func(i, j int) bool {
				return related[i].Weight > related[j].Weight
			})

			// Injection des suggestions dans les résultats
			for _, rw := range related {
				if int64(len(finalTags)) >= limit {
					break
				}
				if !seen[rw.Tag] {
					finalTags = append(finalTags, rw.Tag)
					seen[rw.Tag] = true
				}
			}
		}
	}

	// 3. REMPLISSAGE (Fallback) : On complète avec la suite alphabétique si on n'a pas atteint la limite
	for _, t := range lexTags {
		if int64(len(finalTags)) >= limit {
			break
		}
		if !seen[t] {
			finalTags = append(finalTags, t)
			seen[t] = true
		}
	}

	// 4. Protection JSON (Garantir un [] vide plutôt qu'un null si le tableau est vide)
	if finalTags == nil {
		finalTags = make([]string, 0)
	}

	return search_models.AutocompleteTagOutput{Tags: finalTags}, nil
}
