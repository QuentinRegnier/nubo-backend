package cache_service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

// AddUserToSpeedCache insère un nouvel utilisateur dans l'index de recherche et le store SPEED cache
func AddUserToSpeedCache(ctx context.Context, u auth_models.UserPayload) error {
	// 1. Insertion dans l'index lexicographique (Score à 0 pour le tri par chaînes)
	lexValue := fmt.Sprintf("%s:%d", strings.ToLower(u.Username), u.ID)
	if err := redis.ZAdd(ctx, "speed_cache:search:lex", 0, lexValue); err != nil {
		return fmt.Errorf("failed to index user lexically in speed cache: %w", err)
	}

	// 2. Construction de la structure d'empreinte minimale (Lite)
	userLite := models.UserLiteRequest{
		ID:               u.ID,
		Username:         u.Username,
		FirstName:        u.FirstName,
		LastName:         u.LastName,
		ProfilePictureID: u.ProfilePictureID,
		Bio:              u.Bio,
		Grade:            u.Grade,
		Badges:           u.Badges,
	}

	// 3. Sérialisation et persistance de l'objet compact
	if err := redis.UsersLite.SetObject(ctx, u.ID, userLite); err != nil {
		return fmt.Errorf("failed to store user lite object in speed cache: %w", err)
	}
	return nil
}

// SearchUserByPrefix recherche des utilisateurs via l'auto-complétion (SPEED Cache)

// SearchUserByPrefix recherche des utilisateurs via l'auto-complétion (SPEED Cache)
func SearchUserByPrefix(ctx context.Context, prefix string, limit int64) ([]models.UserLiteRequest, error) {
	// 1. Recherche ultra-rapide dans l'index lexicographique (O(log(N)))
	lexResults, err := redis.ZRangeByLex(ctx, "speed_cache:search:lex", strings.ToLower(prefix), limit)
	if err != nil {
		return nil, err
	}

	if len(lexResults) == 0 {
		return []models.UserLiteRequest{}, nil
	}

	// 2. Extraction des IDs
	var ids []int64
	for _, res := range lexResults {
		// Le format stocké est "pseudo:id"
		parts := strings.Split(res, ":")
		if len(parts) == 2 {
			if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
	}

	// 3. Hydratation via MGET sur la collection UsersLite
	getRes, err := redis.UsersLite.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}

	var users []models.UserLiteRequest
	// 4. On boucle sur ids pour conserver l'ordre alphabétique exact renvoyé par l'index
	for _, id := range ids {
		if data, ok := getRes.Found[id]; ok {
			var u models.UserLiteRequest
			if err := msgpack.Unmarshal(data, &u); err == nil {
				users = append(users, u)
			}
		}
	}

	// Si un ID manque dans le cache_service (MissingIDs), on l'ignore silencieusement.
	// Pour de l'auto-complétion, la vitesse prime sur l'exhaustivité absolue.
	return users, nil
}

// GetUserLite récupère l'empreinte minimale d'un utilisateur depuis le SPEED Cache (L1)
// avec un fallback en cascade étanche : L2 (MongoDB) -> L3 (PostgreSQL) et réhydratation automatique.
func GetUserLite(ctx context.Context, userID int64) (models.UserLiteRequest, error) {
	var ul models.UserLiteRequest

	// 1. TENTATIVE L1 : SPEED Cache (Redis UsersLite)
	err := redis.UsersLite.GetObject(ctx, userID, &ul)
	if err == nil {
		return ul, nil
	}

	// 2. FALLBACK L2 : Cold Storage (MongoDB)
	uMongo, errMongo := mongo.MongoLoadUser(userID, "", "", "")
	if errMongo == nil {
		// Réhydratation L1 synchrone
		_ = AddUserToSpeedCache(ctx, uMongo)
		return models.UserLiteRequest{
			ID:               uMongo.ID,
			Username:         uMongo.Username,
			FirstName:        uMongo.FirstName,
			LastName:         uMongo.LastName,
			ProfilePictureID: uMongo.ProfilePictureID,
			Bio:              uMongo.Bio,
			Grade:            uMongo.Grade,
			Badges:           uMongo.Badges,
		}, nil
	}

	// 3. FALLBACK L3 : Source de Vérité Absolue (PostgreSQL)
	uPg, errPg := postgres.FuncLoadUser(userID, "", "", "")
	if errPg == nil {
		// A. Réhydratation du stockage à froid L2 (MongoDB)
		_ = mongo.MongoUpsertUser(uPg)

		// B. Réhydratation du SPEED Cache L1 (Redis)
		_ = AddUserToSpeedCache(ctx, uPg)

		return models.UserLiteRequest{
			ID:               uPg.ID,
			Username:         uPg.Username,
			FirstName:        uPg.FirstName,
			LastName:         uPg.LastName,
			ProfilePictureID: uPg.ProfilePictureID,
			Bio:              uPg.Bio,
			Grade:            uPg.Grade,
			Badges:           uPg.Badges,
		}, nil
	}

	return ul, fmt.Errorf("impossible de trouver l'utilisateur %d dans les couches d'infrastructure", userID)
}
