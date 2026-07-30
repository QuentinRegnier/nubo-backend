package auth_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// Logout déconnecte l'appareil courant en utilisant le deviceToken du JWT.
func Logout(ctx context.Context, callerID int64, deviceToken string) error {
	if deviceToken == "" {
		return nil // Rien à déconnecter
	}

	// 1. Récupération de la session courante (L1 -> L3) via l'index DeviceToken
	session, err := cache_service.LoadSessionFromCache(ctx, callerID, deviceToken, "")
	if err != nil || session.ID == 0 {
		sessionsPg, errPg := postgres.FuncLoadSession(-1, callerID, deviceToken, "")
		if errPg != nil || sessionsPg.ID == 0 {
			// Idempotence : Si introuvable, on considère l'utilisateur déjà déconnecté
			return nil
		}
		session = sessionsPg
	}

	// 2. Destruction en RAM (L1) via le Service Cache (Respect DDD)
	_ = cache_service.DeleteSessionFromCache(ctx, session.ID, callerID, deviceToken)

	// 3. Persistance asynchrone (Envoi de la demande de Delete aux workers)
	payload := models.SessionsRequest{ID: session.ID, UserID: callerID}
	return redis.EnqueueDB(ctx, session.ID, callerID, redis.EntitySession, redis.ActionDelete, payload, redis.TargetAll)
}
