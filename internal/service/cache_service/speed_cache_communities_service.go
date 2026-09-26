package cache_service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/vmihailenco/msgpack/v5"
)

// ############################################################################
// # SERVICE : SPEED CACHE (COMMUNAUTÉS)
// ############################################################################

// StoreCommunityLiteInSpeedCache sauvegarde un objet CommunityLiteRequest
// et met à jour l'index Lexicographique pour l'autocomplétion.
func StoreCommunityLiteInSpeedCache(ctx context.Context, communityLitePayload lite_models.CommunityLiteRequest) error {
	lexicographicValue := fmt.Sprintf("%s:%d", strings.ToLower(communityLitePayload.Name), communityLitePayload.ID)

	errZAdd := redis.CommunitiesLex.ZAdd(ctx, variables.LexicographicGlobalKey, 0, lexicographicValue)
	if errZAdd != nil {
		logger.Log.Error().Err(errZAdd).Int64("community_id", communityLitePayload.ID).Msg("Impossible d'indexer la communauté dans le dictionnaire Lexicographique")
		return nubo_error.NewInternal()
	}

	errSet := redis.SpeedCommunity.SetObject(ctx, communityLitePayload.ID, communityLitePayload)
	if errSet != nil {
		logger.Log.Error().Err(errSet).Int64("community_id", communityLitePayload.ID).Msg("Impossible de sauvegarder la communauté dans l'Object Cache")
		return nubo_error.NewInternal()
	}

	return nil
}

// UpdateCommunityMemberCountInSpeedCache met à jour le compteur de membres en RAM (O(1)).
func UpdateCommunityMemberCountInSpeedCache(ctx context.Context, communityID int64, delta int) {
	if delta == 0 {
		return
	}

	var communityLitePayload lite_models.CommunityLiteRequest

	// On modifie silencieusement seulement si la communauté est bien présente en L1
	errGet := redis.SpeedCommunity.GetObject(ctx, communityID, &communityLitePayload)
	if errGet == nil && communityLitePayload.ID != 0 {
		communityLitePayload.MemberCount += delta
		if communityLitePayload.MemberCount < 0 {
			communityLitePayload.MemberCount = 0
		}

		errSet := redis.SpeedCommunity.SetObject(ctx, communityLitePayload.ID, communityLitePayload)
		if errSet != nil {
			logger.Log.Warn().Err(errSet).Int64("community_id", communityID).Msg("Échec de la mise à jour du compteur de membres en L1")
		}
	}
}

// SearchCommunitiesByPrefix recherche des communautés en O(log(N)) RAM et les réhydrate.
func SearchCommunitiesByPrefix(ctx context.Context, searchPrefix string, resultLimit int64) ([]lite_models.CommunityLiteRequest, error) {

	// 1. Recherche ultra-rapide dans l'index lexicographique
	lexicographicResultsList, errLex := redis.CommunitiesLex.ZRangeByLex(ctx, variables.LexicographicGlobalKey, strings.ToLower(searchPrefix), resultLimit)
	if errLex != nil {
		logger.Log.Error().Err(errLex).Str("prefix", searchPrefix).Msg("Erreur lors du ZRangeByLex des communautés")
		return nil, nubo_error.NewInternal()
	}

	if len(lexicographicResultsList) == 0 {
		return []lite_models.CommunityLiteRequest{}, nil
	}

	// 2. Extraction des IDs
	var extractedCommunityIDs []int64
	for _, lexResultString := range lexicographicResultsList {
		// Le format stocké est "nom:id"
		lexParts := strings.Split(lexResultString, ":")
		if len(lexParts) == 2 {
			if parsedID, errParse := strconv.ParseInt(lexParts[1], 10, 64); errParse == nil {
				extractedCommunityIDs = append(extractedCommunityIDs, parsedID)
			}
		}
	}

	// 3. Hydratation via MGET sur la collection SpeedCommunity
	multiGetResult, errMGet := redis.SpeedCommunity.GetMany(ctx, extractedCommunityIDs)
	if errMGet != nil {
		logger.Log.Error().Err(errMGet).Msg("Erreur lors de l'hydratation massive des communautés depuis le Speed Cache")
		return nil, nubo_error.NewInternal()
	}

	var hydratedCommunitiesList []lite_models.CommunityLiteRequest

	// 4. On boucle sur l'array originel pour conserver l'ordre alphabétique exact renvoyé par l'index
	for _, requestedID := range extractedCommunityIDs {
		if binaryData, isFound := multiGetResult.Found[requestedID]; isFound {
			var communityLitePayload lite_models.CommunityLiteRequest
			if errUnmarshal := msgpack.Unmarshal(binaryData, &communityLitePayload); errUnmarshal == nil {
				hydratedCommunitiesList = append(hydratedCommunitiesList, communityLitePayload)
			}
		}
	}

	return hydratedCommunitiesList, nil
}
