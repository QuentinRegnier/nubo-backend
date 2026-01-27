package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables" // Pour ToleranceTimeSeconds
)

// DeriveNextSecret (Inchangé)
func DeriveNextSecret(secretCurrent, secretLast, masterToken, deviceToken string) string {
	data := secretCurrent + secretLast + masterToken + deviceToken
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// RotateRatchet effectue la rotation atomique et sécurisée
func RotateRatchet(ctx context.Context, userID int64, clientCurrentSecret string, incomingJWT string) error {
	sessionRaw, err := redis.RedisLoadSession(userID, "", "", clientCurrentSecret)
	if err != nil {
		return errors.New("session introuvable ou secret invalide")
	}
	var newCurrentSecret, newLastSecret string
	if sessionRaw.CurrentSecret != "" && sessionRaw.LastSecret != "" {
		newCurrentSecret = DeriveNextSecret(sessionRaw.CurrentSecret, sessionRaw.LastSecret, sessionRaw.MasterToken, sessionRaw.DeviceToken)
		newLastSecret = sessionRaw.CurrentSecret
	} else {
		newCurrentSecret = DeriveNextSecret(sessionRaw.DeviceToken, sessionRaw.MasterToken, sessionRaw.MasterToken, sessionRaw.DeviceToken)
		newLastSecret = sessionRaw.DeviceToken
	}
	sessionRaw.CurrentSecret = newCurrentSecret
	sessionRaw.LastSecret = newLastSecret
	sessionRaw.LastJWT = incomingJWT
	sessionRaw.ToleranceTime = time.Now().Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)
	if err := redis.RedisUpdateSession(sessionRaw); err != nil {
		return err
	}
	return redis.EnqueueDB(ctx, sessionRaw.ID, 0, redis.EntitySession, redis.ActionUpdate, sessionRaw, redis.TargetMongo)
}

// ResetRatchet effectue un hard-reset de la session (Changement MasterToken)
func ResetRatchet(ctx context.Context, sessionID int64, newMasterToken, deviceToken, oldJWT string) (string, error) {
	if newMasterToken == "" || deviceToken == "" {
		return "", errors.New("ResetRatchet: newMasterToken et deviceToken ne peuvent pas être vides")
	}
	firstDerivedSecret := DeriveNextSecret(deviceToken, newMasterToken, newMasterToken, deviceToken)
	return firstDerivedSecret, nil
}
