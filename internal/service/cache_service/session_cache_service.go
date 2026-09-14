package cache_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
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
func SetSessionInCache(ctx context.Context, s auth_models.SessionsPayload) error {
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
func LoadSessionFromCache(ctx context.Context, userID int64, firebaseInstallationID string, masterToken string) (auth_models.SessionsPayload, error) {
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
		return auth_models.SessionsPayload{}, nubo_error.NewNotFound("SESSION_NOT_FOUND", "Session introuvable en RAM.", nil)
	}

	var s auth_models.SessionsPayload
	if err := redis.Sessions.GetObject(c, targetID, &s); err != nil {
		return auth_models.SessionsPayload{}, err // C'est une erreur d'infrastructure, on la propage
	}

	if masterToken != "" && s.MasterToken != masterToken {
		return auth_models.SessionsPayload{}, nubo_error.NewForbidden("INVALID_MASTER_TOKEN", "Jeton maître invalide.", nil)
	}

	return s, nil
}

// DeleteSessionFromCache supprime la session, son index, et pose un verrou de révocation L1
func DeleteSessionFromCache(ctx context.Context, sessionID int64, userID int64, firebaseInstallationID string) error {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	if userID != 0 && firebaseInstallationID != "" {
		idxKey := fmt.Sprintf("%d:%s", userID, firebaseInstallationID)

		// 1. POSE DU TOMBSTONE VIA L'ABSTRACTION DDD
		// On inscrit la clé composite dans la collection Blacklist (TTL automatique géré par le Manager)
		_ = redis.SessionBlacklist.SetPrimitive(c, idxKey, "1")

		// 2. Suppression de l'index de recherche normal
		_ = redis.SessionIndexes.DeletePrimitive(c, idxKey)
	}

	// 3. Suppression de l'objet principal
	_ = redis.Sessions.DeleteObject(c, sessionID)

	return nil
}

// GetFirebaseInstallationIDsCascade récupère tous les tokens d'appareils de l'utilisateur (L1 -> L2 -> L3)
// L'architecture repose désormais sur l'abstraction Collection (DDD).
func GetFirebaseInstallationIDsCascade(ctx context.Context, userID int64) ([]string, error) {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	// 1. TENTATIVE L1 (Redis via SMembers)
	// On utilise l'abstraction DDD de ta Collection pour récupérer le SET des tokens.
	// (Note: Cela implique que lors du login, tu fasses : redis.SessionIndexes.SAdd(ctx, userID, firebaseID))
	fids, err := redis.SessionIndexes.SMembers(c, userID)
	if err == nil && len(fids) > 0 {
		return fids, nil
	}

	// 2. FALLBACK L2 (MongoDB)
	fids, err = mongo.MongoGetFirebaseInstallationIDs(userID)
	if err == nil && len(fids) > 0 {
		// ⬆️ AUTO-GUÉRISON L1 : On répare le cache RAM
		go healSessionIndexL1(userID, fids)
		return fids, nil
	}

	// 3. FALLBACK L3 COMPLET (PostgreSQL) avec réhydratation
	sessionsPg, errPg := postgres.FuncLoadAllUserSessions(ctx, userID)
	if errPg == nil && len(sessionsPg) > 0 {
		var newFids []string

		for _, session := range sessionsPg {
			newFids = append(newFids, session.FirebaseInstallationID)

			// PROMOTION L3 -> L2 & L1
			go func(s auth_models.SessionsPayload) {
				bgCtx := context.Background()
				// L1 : Hydratation en RAM de l'objet complet
				_ = SetSessionInCache(bgCtx, s)

				// L2 : Asynchrone vers Mongo
				_ = redis.EnqueueDB(bgCtx, s.ID, s.UserID, redis.EntitySession, redis.ActionUpdate, s, redis.TargetMongo)
			}(session)
		}

		// ⬆️ AUTO-GUÉRISON L1 : On répare l'index de recherche (Le Set)
		go healSessionIndexL1(userID, newFids)

		return newFids, nil
	}

	return nil, nubo_error.NewNotFound("NO_ACTIVE_DEVICE", "Aucun appareil actif trouvé pour cet utilisateur.", nil)
}

// healSessionIndexL1 est un helper local asynchrone pour insérer un lot de FIDs dans le Set L1
func healSessionIndexL1(userID int64, fids []string) {
	bgCtx := context.Background()
	// Conversion en slice de "any" pour l'interface SAdd
	args := make([]any, len(fids))
	for i, fid := range fids {
		args[i] = fid
	}

	// Utilisation de ton abstraction DDD
	_ = redis.SessionIndexes.SAdd(bgCtx, userID, args...)
	_ = redis.SessionIndexes.RefreshTTL(bgCtx, userID)
}
