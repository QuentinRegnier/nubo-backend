package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

func DeriveNextSecret(secretCurrent, secretLast, masterToken, firebaseInstallationID string) string {
	data := secretCurrent + secretLast + masterToken + firebaseInstallationID
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// RotateRatchet effectue la rotation atomique et sécurisée avec Cascade L1->L2->L3
func RotateRatchet(ctx context.Context, userID int64, firebaseInstallationID string, clientCurrentSecret string, incomingJWT string) error {
	// 0. BOUCLIER TOMBSTONE (O(1) en RAM)
	idxKey := fmt.Sprintf("%d:%s", userID, firebaseInstallationID)
	if isRevoked, _ := redis.SessionBlacklist.Exists(ctx, idxKey); isRevoked {
		return nubo_error.NewForbidden("SESSION_REVOKED", "Session révoquée (déconnexion en cours).", nil)
	}

	// 1. CASCADE L1 -> L2 -> L3 (Recherche sécurisée)
	sessionRaw, err := cache_service.LoadSessionFromCache(ctx, userID, firebaseInstallationID, "")
	if err != nil || sessionRaw.ID == 0 {
		sessionRaw, err = mongo.MongoLoadSession(userID, firebaseInstallationID, "", "")
		if err != nil || sessionRaw.ID == 0 {
			sessionRaw, err = postgres.FuncLoadSession(-1, userID, firebaseInstallationID, "")
			if err != nil || sessionRaw.ID == 0 {
				return nubo_error.NewNotFound("SESSION_NOT_FOUND", "Session introuvable.", err)
			}
		}
	}

	// 2. VÉRIFICATION DE SYNCHRONISATION
	if sessionRaw.CurrentSecret != clientCurrentSecret {
		return nubo_error.NewForbidden("INVALID_SECRET", "Désynchronisation de sécurité détectée.", nil)
	}

	// 3. ROTATION CRYPTOGRAPHIQUE
	var newCurrentSecret, newLastSecret string
	if sessionRaw.CurrentSecret != "" && sessionRaw.LastSecret != "" {
		newCurrentSecret = DeriveNextSecret(sessionRaw.CurrentSecret, sessionRaw.LastSecret, sessionRaw.MasterToken, sessionRaw.FirebaseInstallationID)
		newLastSecret = sessionRaw.CurrentSecret
	} else {
		newCurrentSecret = DeriveNextSecret(sessionRaw.FirebaseInstallationID, sessionRaw.MasterToken, sessionRaw.MasterToken, sessionRaw.FirebaseInstallationID)
		newLastSecret = sessionRaw.FirebaseInstallationID
	}

	sessionRaw.CurrentSecret = newCurrentSecret
	sessionRaw.LastSecret = newLastSecret
	sessionRaw.LastJWT = incomingJWT
	sessionRaw.ToleranceTime = time.Now().Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)

	// 4. SAUVEGARDE EN RAM (L1)
	_ = cache_service.SetSessionInCache(ctx, sessionRaw)

	// 5. PERSISTANCE ASYNCHRONE (L2 & L3)
	return redis.EnqueueDB(ctx, sessionRaw.ID, userID, redis.EntitySession, redis.ActionUpdate, sessionRaw, redis.TargetAll)
}

// ResetRatchet génère le premier secret dérivé suite à un Hard Reset (Rotation du MasterToken)
func ResetRatchet(newMasterToken, firebaseInstallationID string) (string, error) {
	if newMasterToken == "" || firebaseInstallationID == "" {
		return "", nubo_error.NewBadRequest("MISSING_PARAMS", "Paramètres manquants pour la rotation.", nil)
	}
	firstDerivedSecret := DeriveNextSecret(firebaseInstallationID, newMasterToken, newMasterToken, firebaseInstallationID)
	return firstDerivedSecret, nil
}
