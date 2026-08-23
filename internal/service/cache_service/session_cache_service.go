package cache_service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// Helper local pour protéger l'API des blocages si Redis ne répond pas
func getShortCtx(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, 2*time.Second)
}

// SetSessionInCache sauvegarde la session et son index de recherche
func SetSessionInCache(ctx context.Context, s models.SessionsRequest) error {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	if err := redis.Sessions.SetObject(c, s.ID, s); err != nil {
		return err
	}

	if s.UserID != 0 && s.FirebaseInstallationID != "" {
		// Le préfixe "session_cache:" est maintenant automatiquement géré par la Collection
		idxKey := fmt.Sprintf("%d:%s", s.UserID, s.FirebaseInstallationID)
		return redis.SessionIndexes.SetPrimitive(c, idxKey, s.ID)
	}

	return nil
}

// LoadSessionFromCache charge une session.
func LoadSessionFromCache(ctx context.Context, userID int64, firebaseInstallationID string, masterToken string) (models.SessionsRequest, error) {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	var targetID int64

	if userID != -1 && firebaseInstallationID != "" {
		idxKey := fmt.Sprintf("%d:%s", userID, firebaseInstallationID)
		val, err := redis.SessionIndexes.GetInt64(c, idxKey)
		if err == nil {
			targetID = val
		}
	}

	if targetID == 0 {
		return models.SessionsRequest{}, fmt.Errorf("session introuvable dans redis (index miss)")
	}

	var s models.SessionsRequest
	if err := redis.Sessions.GetObject(c, targetID, &s); err != nil {
		return models.SessionsRequest{}, err
	}

	if masterToken != "" && s.MasterToken != masterToken {
		return models.SessionsRequest{}, fmt.Errorf("master token mismatch")
	}

	return s, nil
}

// DeleteSessionFromCache supprime la session et son index de recherche du cache L1
func DeleteSessionFromCache(ctx context.Context, sessionID int64, userID int64, firebaseInstallationID string) error {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	// 1. Suppression de l'objet principal
	_ = redis.Sessions.DeleteObject(c, sessionID)

	// 2. Suppression de l'index de recherche associé
	if userID != 0 && firebaseInstallationID != "" {
		idxKey := fmt.Sprintf("%d:%s", userID, firebaseInstallationID)
		_ = redis.SessionIndexes.DeletePrimitive(c, idxKey)
	}

	return nil
}

// GetFirebaseInstallationIDsCascade récupère tous les tokens d'appareils de l'utilisateur (L1 -> L2 -> L3)
func GetFirebaseInstallationIDsCascade(ctx context.Context, userID int64) ([]string, error) {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	var fids []string

	// 1. TENTATIVE L1 (Redis via KEYS sur SessionIndexes)
	// La clé est au format session_cache:<userID>:<firebaseInstallationID>
	pattern := fmt.Sprintf("%s:%d:*", redis.SessionIndexes.Prefix, userID)
	keys, err := redis.Keys(c, pattern)
	if err == nil && len(keys) > 0 {
		for _, key := range keys {
			parts := strings.Split(key, ":")
			if len(parts) >= 3 {
				fids = append(fids, parts[2])
			}
		}
		return fids, nil
	}

	// 2. FALLBACK L2 (MongoDB)
	fids, err = mongo.MongoGetFirebaseInstallationIDs(userID)
	if err == nil && len(fids) > 0 {
		return fids, nil
	}

	// 3. FALLBACK L3 (PostgreSQL)
	fids, err = postgres.FuncGetFirebaseInstallationIDs(ctx, userID)
	if err == nil && len(fids) > 0 {
		return fids, nil
	}

	return nil, fmt.Errorf("aucun appareil actif trouvé pour l'utilisateur")
}
