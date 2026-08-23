package cache_service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// GetMessageIDsFromSpeedCache résout l'index en L1, en appliquant le Plafond Temporel (frozenID)
func GetMessageIDsFromSpeedCache(ctx context.Context, convID int64, offsetID int64, limit int64, direction string, frozenID int64) ([]int64, error) {
	var idsStr []string
	var err error

	// 1. TENTATIVE L1 (ZSET)
	if direction == "top" { // Chargement de l'historique (plus anciens)
		maxScore := "+inf"

		// RÈGLE DE GEL : Si un plafond existe, c'est notre maximum absolu
		if frozenID > 0 {
			maxScore = strconv.FormatInt(frozenID, 10)
		}

		// GESTION DU SCROLL : Si on a un offset, il remplace le plafond SEULEMENT s'il est plus petit
		if offsetID > 0 {
			if frozenID > 0 && offsetID > frozenID {
				// L'utilisateur essaie de tricher en scrollant depuis un ID du futur, on le cap au gel !
				maxScore = strconv.FormatInt(frozenID, 10)
			} else {
				maxScore = fmt.Sprintf("(%d", offsetID) // Exclusif
			}
		}

		idsStr, err = redis.MessagesIndex.ZRevRangeByScore(ctx, convID, maxScore, "-inf", limit)
	} else { // Nouveaux messages (plus récents)
		minScore := "-inf"
		if offsetID > 0 {
			minScore = fmt.Sprintf("(%d", offsetID) // Exclusif
		}

		maxScore := "+inf"
		if frozenID > 0 {
			maxScore = strconv.FormatInt(frozenID, 10) // Ne doit pas dépasser le gel !
		}

		idsStr, err = redis.MessagesIndex.ZRangeByScore(ctx, convID, minScore, maxScore, limit)
	}

	// 2. VÉRIFICATION DU CACHE HOLE
	if err != nil || len(idsStr) < int(limit) {
		// L3 FALLBACK ABSOLU
		pgIDs, errPg := postgres.FuncLoadMessageIDsPaginated(ctx, convID, offsetID, limit, direction, frozenID)
		if errPg != nil {
			return nil, errPg
		}

		// RÉHYDRATATION DE L'INDEX L1 (Auto-guérison)
		if len(pgIDs) > 0 {
			go func(ids []int64) {
				bgCtx := context.Background()
				for _, id := range ids {
					_ = redis.MessagesIndex.ZAdd(bgCtx, convID, float64(id), strconv.FormatInt(id, 10))
				}
				_ = redis.MessagesIndex.RefreshTTL(bgCtx, convID)
			}(pgIDs)
		}
		return pgIDs, nil
	}

	// Parsing des IDs L1
	var finalIDs []int64
	for _, s := range idsStr {
		if id, e := strconv.ParseInt(s, 10, 64); e == nil {
			finalIDs = append(finalIDs, id)
		}
	}
	return finalIDs, nil
}
