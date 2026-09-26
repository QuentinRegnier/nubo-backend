package cache_service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : SPEED CACHE (MESSAGES ET TIMELINE)
// ############################################################################

// GetMessageIDsFromSpeedCache résout l'index en L1 (ZSET), en appliquant
// le Plafond Temporel (frozenID) pour gérer la pagination bidirectionnelle.
func GetMessageIDsFromSpeedCache(ctx context.Context, conversationID int64, offsetMessageID int64, paginationLimit int64, scrollDirection string, frozenMessageID int64) ([]int64, error) {
	var idStringsList []string
	var errRedis error

	// ── ÉTAPE 1 : TENTATIVE L1 (ZSET) ───────────────────────────────────────

	if scrollDirection == "top" {
		// Chargement de l'historique (les plus anciens)
		maxScoreThreshold := "+inf"

		// RÈGLE DE GEL : Si un plafond existe, c'est notre maximum absolu
		if frozenMessageID > 0 {
			maxScoreThreshold = strconv.FormatInt(frozenMessageID, 10)
		}

		// GESTION DU SCROLL : L'offset remplace le plafond SEULEMENT s'il est plus petit
		if offsetMessageID > 0 {
			if frozenMessageID > 0 && offsetMessageID > frozenMessageID {
				// Anti-triche : L'utilisateur scrolle depuis un ID du futur, on le cap au gel !
				maxScoreThreshold = strconv.FormatInt(frozenMessageID, 10)
			} else {
				// Parenthèse ouverte = borne exclusive dans la syntaxe Redis
				maxScoreThreshold = fmt.Sprintf("(%d", offsetMessageID)
			}
		}

		idStringsList, errRedis = redis.MessagesIndex.ZRevRangeByScore(ctx, conversationID, maxScoreThreshold, "-inf", paginationLimit)

	} else {
		// Nouveaux messages (les plus récents)
		minScoreThreshold := "-inf"
		if offsetMessageID > 0 {
			minScoreThreshold = fmt.Sprintf("(%d", offsetMessageID) // Exclusif
		}

		maxScoreThreshold := "+inf"
		if frozenMessageID > 0 {
			maxScoreThreshold = strconv.FormatInt(frozenMessageID, 10) // Ne doit jamais dépasser le gel
		}

		idStringsList, errRedis = redis.MessagesIndex.ZRangeByScore(ctx, conversationID, minScoreThreshold, maxScoreThreshold, paginationLimit)
	}

	// ── ÉTAPE 2 : VÉRIFICATION DU CACHE HOLE ET FALLBACK L3 ─────────────────

	if errRedis != nil || len(idStringsList) < int(paginationLimit) {

		if errRedis != nil {
			logger.Log.Warn().Err(errRedis).Msg("Erreur L1 lors de la lecture de l'index des messages")
		}

		// FALLBACK ABSOLU L3 (Cold Storage)
		postgresIDsList, errPg := postgres.FuncLoadMessageIDsPaginated(ctx, conversationID, offsetMessageID, paginationLimit, scrollDirection, frozenMessageID)
		if errPg != nil {
			logger.Log.Error().Err(errPg).Int64("conv_id", conversationID).Msg("Erreur L3 lors de la récupération paginée des messages")
			return nil, nubo_error.NewInternal()
		}

		// RÉHYDRATATION DE L'INDEX L1 (Auto-guérison massive et asynchrone)
		if len(postgresIDsList) > 0 {
			go func(repairedIDs []int64) {
				backgroundCtx := context.Background()
				for _, repairedID := range repairedIDs {
					_ = redis.MessagesIndex.ZAdd(backgroundCtx, conversationID, float64(repairedID), strconv.FormatInt(repairedID, 10))
				}
				_ = redis.MessagesIndex.RefreshTTL(backgroundCtx, conversationID)
			}(postgresIDsList)
		}

		return postgresIDsList, nil
	}

	// ── ÉTAPE 3 : PARSING DES IDs TROUVÉS EN L1 ─────────────────────────────

	var finalParsedIDs []int64
	for _, idString := range idStringsList {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			finalParsedIDs = append(finalParsedIDs, parsedID)
		}
	}

	return finalParsedIDs, nil
}
