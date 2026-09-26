package cache_service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/vmihailenco/msgpack/v5"
)

// ############################################################################
// # SERVICE : SPEED CACHE (PROFILS UTILISATEURS ALLÉGÉS)
// ############################################################################

// StoreUserLiteInSpeedCache sauvegarde directement un objet UserLiteRequest
// et met à jour l'index Lexicographique pour la recherche et l'autocomplétion.
func StoreUserLiteInSpeedCache(ctx context.Context, userLitePayload lite_models.UserLiteRequest) error {

	// 1. Insertion dans l'index lexicographique ("pseudo:id")
	lexicographicValue := fmt.Sprintf("%s:%d", strings.ToLower(userLitePayload.Username), userLitePayload.ID)
	errLex := redis.UsersLex.ZAdd(ctx, variables.LexicographicGlobalKey, 0, lexicographicValue)
	if errLex != nil {
		logger.Log.Warn().Err(errLex).Msg("Échec de l'indexation lexicographique d'un utilisateur")
	}

	// 2. Sauvegarde dans l'Object Cache Rapide L1
	errSet := redis.UsersLite.SetObject(ctx, userLitePayload.ID, userLitePayload)
	if errSet != nil {
		logger.Log.Error().Err(errSet).Msg("Échec de la sauvegarde du UserLiteRequest dans le Speed Cache")
		return nubo_error.NewInternal()
	}

	return nil
}

// AddUserToSpeedCache insère un nouvel utilisateur (à la création de compte) dans
// l'index de recherche et le store SPEED cache L1.
func AddUserToSpeedCache(ctx context.Context, userPayload auth_models.UserPayload, settingsPayload user_settings_models.UserSettingsPayload) error {

	lexicographicValue := fmt.Sprintf("%s:%d", strings.ToLower(userPayload.Username), userPayload.ID)
	_ = redis.UsersLex.ZAdd(ctx, variables.LexicographicGlobalKey, 0, lexicographicValue)

	constructedUserLite := lite_models.UserLiteRequest{
		ID:                     userPayload.ID,
		Username:               userPayload.Username,
		FirstName:              userPayload.FirstName,
		LastName:               userPayload.LastName,
		ProfilePictureID:       userPayload.ProfilePictureID,
		Bio:                    userPayload.Bio,
		Grade:                  userPayload.Grade,
		Badges:                 userPayload.Badges,
		ConversationPermission: settingsPayload.Privacy.ConversationPermission,
		AddGroupPermission:     settingsPayload.Privacy.AddGroupPermission,
		HideConnections:        settingsPayload.Privacy.HideConnections,
		ShowOnlineStatus:       settingsPayload.Privacy.ShowOnlineStatus,
		AllowTagging:           settingsPayload.Privacy.AllowTagging,
		AllowMentions:          settingsPayload.Privacy.AllowMentions,
	}

	errSet := redis.UsersLite.SetObject(ctx, userPayload.ID, constructedUserLite)
	if errSet != nil {
		return nubo_error.NewInternal()
	}

	return nil
}

// UpdateUserSpeedCachePrivacy synchronise les changements de confidentialité directement
// en RAM pour un effet immédiat sur les autres utilisateurs.
func UpdateUserSpeedCachePrivacy(ctx context.Context, userID int64, conversationPerm int, addGroupPerm int, hideConnectionsPerm int) error {
	var userLitePayload lite_models.UserLiteRequest

	errGet := redis.UsersLite.GetObject(ctx, userID, &userLitePayload)
	if errGet == nil && userLitePayload.ID != 0 {
		userLitePayload.ConversationPermission = conversationPerm
		userLitePayload.AddGroupPermission = addGroupPerm
		userLitePayload.HideConnections = hideConnectionsPerm

		errSet := redis.UsersLite.SetObject(ctx, userID, userLitePayload)
		if errSet != nil {
			return nubo_error.NewInternal()
		}
	}
	return nil
}

// SearchUserByPrefix recherche des utilisateurs via l'autocomplétion.
func SearchUserByPrefix(ctx context.Context, searchPrefix string, searchLimit int64) ([]lite_models.UserLiteRequest, error) {

	// 1. Recherche ultra-rapide dans l'index lexicographique
	lexicographicResultsList, errLex := redis.UsersLex.ZRangeByLex(ctx, variables.LexicographicGlobalKey, strings.ToLower(searchPrefix), searchLimit)
	if errLex != nil {
		logger.Log.Error().Err(errLex).Msg("Erreur L1 lors du ZRangeByLex utilisateurs")
		return nil, nubo_error.NewInternal()
	}

	if len(lexicographicResultsList) == 0 {
		return []lite_models.UserLiteRequest{}, nil
	}

	// 2. Extraction des IDs
	var extractedIDsList []int64
	for _, lexString := range lexicographicResultsList {
		// Le format stocké est "pseudo:id"
		lexParts := strings.Split(lexString, ":")
		if len(lexParts) == 2 {
			if parsedID, errParse := strconv.ParseInt(lexParts[1], 10, 64); errParse == nil {
				extractedIDsList = append(extractedIDsList, parsedID)
			}
		}
	}

	// 3. Hydratation massive via MGET sur la collection UsersLite
	multiGetResult, errMGet := redis.UsersLite.GetMany(ctx, extractedIDsList)
	if errMGet != nil {
		logger.Log.Error().Err(errMGet).Msg("Échec L1 lors de l'hydratation massive des SpeedUsers")
		return nil, nubo_error.NewInternal()
	}

	var hydratedUsersList []lite_models.UserLiteRequest

	// 4. On boucle sur l'array originel pour conserver l'ordre alphabétique
	for _, requestedID := range extractedIDsList {
		if binaryData, isFound := multiGetResult.Found[requestedID]; isFound {
			var userLitePayload lite_models.UserLiteRequest
			if errUnmarshal := msgpack.Unmarshal(binaryData, &userLitePayload); errUnmarshal == nil {
				hydratedUsersList = append(hydratedUsersList, userLitePayload)
			}
		}
	}

	// Si un ID manque dans le cache, on l'ignore silencieusement.
	// Pour de l'autocomplétion, la vitesse prime sur l'exhaustivité absolue.
	return hydratedUsersList, nil
}

// GetUserLite récupère l'empreinte minimale d'un utilisateur depuis le SPEED Cache (L1)
// avec un fallback en cascade L2 (MongoDB) -> L3 (PostgreSQL) et réhydratation automatique.
func GetUserLite(ctx context.Context, userID int64) (lite_models.UserLiteRequest, error) {
	var userLitePayload lite_models.UserLiteRequest

	// ── ÉTAPE 1 : TENTATIVE L1 (SPEED CACHE) ────────────────────────────────
	errL1 := redis.UsersLite.GetObject(ctx, userID, &userLitePayload)
	if errL1 == nil {
		return userLitePayload, nil
	}

	// ── ÉTAPE 2 : FALLBACK L2 (WARM STORAGE MONGODB) ────────────────────────
	userFromMongo, errMongo := mongo.MongoLoadUser(userID, "", "", "")
	if errMongo == nil {
		// Récupération des paramètres de confidentialité L1
		settingsPayload, _ := object_cache_service.GetUserSettingsCascade(ctx, userID)

		// Réhydratation L1 synchrone avec les deux objets combinés
		_ = AddUserToSpeedCache(ctx, userFromMongo, settingsPayload)

		return lite_models.UserLiteRequest{
			ID:                     userFromMongo.ID,
			Username:               userFromMongo.Username,
			FirstName:              userFromMongo.FirstName,
			LastName:               userFromMongo.LastName,
			ProfilePictureID:       userFromMongo.ProfilePictureID,
			Bio:                    userFromMongo.Bio,
			Grade:                  userFromMongo.Grade,
			Badges:                 userFromMongo.Badges,
			ConversationPermission: settingsPayload.Privacy.ConversationPermission,
			AddGroupPermission:     settingsPayload.Privacy.AddGroupPermission,
			HideConnections:        settingsPayload.Privacy.HideConnections,
			ShowOnlineStatus:       settingsPayload.Privacy.ShowOnlineStatus,
			AllowTagging:           settingsPayload.Privacy.AllowTagging,
			AllowMentions:          settingsPayload.Privacy.AllowMentions,
		}, nil
	}

	// ── ÉTAPE 3 : FALLBACK ABSOLU L3 (COLD STORAGE POSTGRESQL) ──────────────
	userFromPg, errPg := postgres.FuncLoadUser(userID, "", "", "")
	if errPg == nil {

		// PROMOTION L3 -> L2 (Write-Behind)
		go func(promotedUser auth_models.UserPayload) {
			backgroundCtx := context.Background()
			_ = redis.EnqueueDB(backgroundCtx, promotedUser.ID, 0, redis.EntityUser, redis.ActionUpdate, promotedUser, redis.TargetMongo)
		}(userFromPg)

		// PROMOTION L3 -> L1
		settingsPayload, _ := object_cache_service.GetUserSettingsCascade(ctx, userID)
		_ = AddUserToSpeedCache(ctx, userFromPg, settingsPayload)

		return lite_models.UserLiteRequest{
			ID:                     userFromPg.ID,
			Username:               userFromPg.Username,
			FirstName:              userFromPg.FirstName,
			LastName:               userFromPg.LastName,
			ProfilePictureID:       userFromPg.ProfilePictureID,
			Bio:                    userFromPg.Bio,
			Grade:                  userFromPg.Grade,
			Badges:                 userFromPg.Badges,
			ConversationPermission: settingsPayload.Privacy.ConversationPermission,
			AddGroupPermission:     settingsPayload.Privacy.AddGroupPermission,
			HideConnections:        settingsPayload.Privacy.HideConnections,
			ShowOnlineStatus:       settingsPayload.Privacy.ShowOnlineStatus,
			AllowTagging:           settingsPayload.Privacy.AllowTagging,
			AllowMentions:          settingsPayload.Privacy.AllowMentions,
		}, nil
	}

	return userLitePayload, nubo_error.NewNotFound(nubo_error.CodeNotFound, "L'utilisateur est introuvable.", nil)
}
