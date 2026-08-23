package auth_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// Logout déconnecte l'appareil courant en utilisant le firebaseInstallationID du JWT.
func Logout(ctx context.Context, callerID int64, firebaseInstallationID string) error {
	if firebaseInstallationID == "" {
		return nil // Rien à déconnecter
	}

	// 1. Récupération de la session courante (L1 -> L3) via l'index FirebaseInstallationID
	session, err := cache_service.LoadSessionFromCache(ctx, callerID, firebaseInstallationID, "")
	if err != nil || session.ID == 0 {
		sessionsPg, errPg := postgres.FuncLoadSession(-1, callerID, firebaseInstallationID, "")
		if errPg != nil || sessionsPg.ID == 0 {
			// Idempotence : Si introuvable, on considère l'utilisateur déjà déconnecté
			return nil
		}
		session = sessionsPg
	}

	// 2. Destruction en RAM (L1) via le Service Cache (Respect DDD)
	_ = cache_service.DeleteSessionFromCache(ctx, session.ID, callerID, firebaseInstallationID)

	// 3. Persistance asynchrone (Envoi de la demande de Delete aux workers)
	payload := models.SessionsRequest{ID: session.ID, UserID: callerID}
	return redis.EnqueueDB(ctx, session.ID, callerID, redis.EntitySession, redis.ActionDelete, payload, redis.TargetAll)
}
