package cache_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
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

	if s.UserID != 0 && s.DeviceToken != "" {
		idxKey := fmt.Sprintf("session_cache:%d:%s", s.UserID, s.DeviceToken)
		return redisgo.Rdb.Set(c, idxKey, s.ID, redis.Sessions.DefaultTTL).Err()
	}

	return nil
}

// LoadSessionFromCache charge une session.
func LoadSessionFromCache(ctx context.Context, userID int64, deviceToken string, masterToken string) (models.SessionsRequest, error) {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	var targetID int64

	if userID != -1 && deviceToken != "" {
		idxKey := fmt.Sprintf("session_cache:%d:%s", userID, deviceToken)
		val, err := redisgo.Rdb.Get(c, idxKey).Int64()
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
