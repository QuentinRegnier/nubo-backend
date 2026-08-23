package auth_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// RevokeSession vérifie la propriété et détruit une session à distance
func RevokeSession(ctx context.Context, callerID int64, sessionID int64) error {
	var s models.SessionsRequest

	// 1. TENTATIVE L1 (RAM) : Récupération pour vérifier la propriété et obtenir le FirebaseInstallationID (pour l'index)
	err := redis.Sessions.GetObject(ctx, sessionID, &s)
	if err != nil || s.ID == 0 {
		// FALLBACK L3 : Si la session n'est plus en RAM, on vérifie en BDD
		// Utilisation de la fonction générique d'origine pour récupérer l'objet complet
		sessionsPg, errPg := postgres.FuncLoadSession(sessionID, callerID, "", "")
		if errPg != nil || sessionsPg.ID == 0 {
			return errors.New("session introuvable ou accès refusé")
		}
		s = sessionsPg
	}

	// 2. SÉCURITÉ ABSOLUE : Vérification de la propriété
	if s.UserID != callerID {
		return errors.New("accès refusé : cette session ne vous appartient pas")
	}

	// 3. PURGE DU CACHE L1 (Object + Index) via le service dédié (Pur DDD)
	_ = cache_service.DeleteSessionFromCache(ctx, sessionID, s.UserID, s.FirebaseInstallationID)

	// 4. ENVOI NOTIFICATION
	_ = realtime_service.DistributeToUsers(ctx, "session.revoked", map[string]int64{"session_id": sessionID}, []int64{callerID})

	// 5. PERSISTANCE ASYNCHRONE : Envoi au Worker pour suppression SQL
	payload := models.SessionsRequest{ID: sessionID, UserID: callerID} // Payload minimal pour le mapping
	return redis.EnqueueDB(ctx, sessionID, callerID, redis.EntitySession, redis.ActionDelete, payload, redis.TargetAll)
}
