package search_service

import (
	"context"
	"sort"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : AUTOCOMPLÉTION DES HASHTAGS
// ############################################################################

// AutocompleteTags orchestre la recherche ultra-rapide de hashtags (O(log N))
// et l'injection de suggestions sémantiques contextuelles.
func AutocompleteTags(ctx context.Context, input search_models.AutocompleteTagInput) (search_models.AutocompleteTagOutput, error) {
	searchLimit := input.Limit
	if searchLimit == 0 {
		searchLimit = variables.DefaultAutocompleteLimit
	}

	sanitizedQuery := strings.ToLower(strings.TrimSpace(input.Query))
	var finalTagResults []string
	alreadySeenTagsMap := make(map[string]bool)

	// ── ÉTAPE 1 : RECHERCHE LEXICOGRAPHIQUE L1 (O(log N)) ───────────────────

	lexicographicTags, errRedis := cache_service.SearchTagsByPrefix(ctx, sanitizedQuery, searchLimit)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Str("query", sanitizedQuery).Msg("Erreur L1 lors de la recherche lexicographique des tags")
		return search_models.AutocompleteTagOutput{}, nubo_error.NewInternal()
	}

	// On vérifie si la requête de l'utilisateur correspond exactement à un tag existant
	isExactTagMatch := false
	for _, tag := range lexicographicTags {
		if tag == sanitizedQuery {
			isExactTagMatch = true
			break
		}
	}

	// ── ÉTAPE 2 : MAGIE SÉMANTIQUE (GRAPHE DE MARKOV) ───────────────────────

	if isExactTagMatch {
		// On force le tag exact en première position absolue pour rassurer l'utilisateur
		finalTagResults = append(finalTagResults, sanitizedQuery)
		alreadySeenTagsMap[sanitizedQuery] = true

		// Interrogation du Cache Sémantique en RAM (O(1))
		semanticallyRelatedTagsMap := cache_service.GetRelatedTagsLazy(ctx, sanitizedQuery)
		if len(semanticallyRelatedTagsMap) > 0 {

			// Structure temporaire pour trier par pertinence algorithmique
			type tagWeightDTO struct {
				Tag    string
				Weight float64
			}

			var relatedTagsList []tagWeightDTO
			for tag, weight := range semanticallyRelatedTagsMap {
				relatedTagsList = append(relatedTagsList, tagWeightDTO{Tag: tag, Weight: weight})
			}

			// Tri décroissant sur la force du lien sémantique
			sort.Slice(relatedTagsList, func(i, j int) bool {
				return relatedTagsList[i].Weight > relatedTagsList[j].Weight
			})

			// Injection des suggestions dans les résultats finaux
			for _, relatedItem := range relatedTagsList {
				if int64(len(finalTagResults)) >= searchLimit {
					break
				}
				if !alreadySeenTagsMap[relatedItem.Tag] {
					finalTagResults = append(finalTagResults, relatedItem.Tag)
					alreadySeenTagsMap[relatedItem.Tag] = true
				}
			}
		}
	}

	// ── ÉTAPE 3 : REMPLISSAGE FALLBACK (ALPHABÉTIQUE) ───────────────────────

	// On complète avec la suite alphabétique issue de la recherche initiale
	// si la limite maximale n'a pas été atteinte.
	for _, tag := range lexicographicTags {
		if int64(len(finalTagResults)) >= searchLimit {
			break
		}
		if !alreadySeenTagsMap[tag] {
			finalTagResults = append(finalTagResults, tag)
			alreadySeenTagsMap[tag] = true
		}
	}

	// ── ÉTAPE 4 : ASSEMBLAGE DU JSON FINAL ──────────────────────────────────

	// Protection JSON stricte : Garantir un array vide [] plutôt qu'un `null`
	if finalTagResults == nil {
		finalTagResults = make([]string, 0)
	}

	return search_models.AutocompleteTagOutput{
		Tags: finalTagResults,
	}, nil
}
