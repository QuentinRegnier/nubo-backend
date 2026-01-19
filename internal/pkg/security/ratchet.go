package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
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
func RotateRatchet(ctx context.Context, userID int, clientCurrentSecret string, incomingJWT string) error {
	// 1. FILTRE STRICT : On cherche par UserID ET par le Secret fourni.
	// Si le secret n'est pas le bon, Redis ne renverra rien, donc erreur.
	filter := map[string]any{
		"user_id":        map[string]any{"$eq": userID},
		"current_secret": map[string]any{"$eq": clientCurrentSecret},
	}

	sessionsData, err := redis.Sessions.Get(ctx, filter)
	if err != nil || len(sessionsData) == 0 {
		return errors.New("session introuvable ou secret invalide")
	}

	// On prend la session trouvée (forcément la bonne grâce au filtre)
	sessionRaw := sessionsData[0]
	var s domain.SessionsRequest
	if err := pkg.ToStruct(sessionRaw, &s); err != nil {
		return err
	}

	// 2. Calcul des Nouveaux Secrets
	var newCurrentSecret, newLastSecret string

	if s.CurrentSecret != "" && s.LastSecret != "" {
		newCurrentSecret = DeriveNextSecret(s.CurrentSecret, s.LastSecret, s.MasterToken, s.DeviceToken)
		newLastSecret = s.CurrentSecret // L'actuel devient le "last" (N)
	} else {
		// Fallback (Recovery)
		newCurrentSecret = DeriveNextSecret(s.DeviceToken, s.MasterToken, s.MasterToken, s.DeviceToken)
		newLastSecret = s.DeviceToken
	}

	// 3. Mise à jour Redis
	updateData := map[string]any{
		"current_secret": newCurrentSecret,
		"last_secret":    newLastSecret,

		// Le token fourni par l'utilisateur devient le "dernier connu" pour la tolérance
		"last_jwt": incomingJWT,

		// On définit la fenêtre de tolérance à partir de MAINTENANT
		"tolerance_time": time.Now().Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second),

		// ON NE TOUCHE PAS A EXPIRES_AT (C'est celle du MasterToken)
	}

	updateFilter := map[string]any{"id": map[string]any{"$eq": s.ID}}
	return redis.Sessions.Update(ctx, updateFilter, updateData)
}

// ResetRatchet effectue un hard-reset de la session (Changement MasterToken)
func ResetRatchet(ctx context.Context, sessionID int, newMasterToken, deviceToken, oldJWT string) (string, error) {
	// Calcul de l'état initial du Ratchet selon tes règles :
	// Secret 0 = MasterToken
	// Secret 1 = DeviceToken
	// Secret 2 (Le premier Current) = f(S1, S0, Master, Device)

	// S0 (Last dans la logique d'init, mais ici Last sera DeviceToken)
	// S1 (DeviceToken)

	// On applique ta règle serveur :
	// current_secret = new_secret (C'est à dire le premier dérivé)
	// last_secret = Device_Token

	// Calcul du premier "Current Secret" utilisable
	if newMasterToken == "" || deviceToken == "" {
		return "", errors.New("ResetRatchet: newMasterToken et deviceToken ne peuvent pas être vides")
	}
	firstDerivedSecret := DeriveNextSecret(deviceToken, newMasterToken, newMasterToken, deviceToken)
	return firstDerivedSecret, nil
}
