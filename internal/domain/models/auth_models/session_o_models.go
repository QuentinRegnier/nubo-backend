package auth_models

import "time"

// SessionView représente la session telle qu'elle sera envoyée au front-end.
// Les données sensibles (MasterToken, FirebaseInstallationID, Secrets) sont physiquement exclues de cette structure.
type SessionView struct {
	ID         int64          `json:"id"`
	DeviceInfo map[string]any `json:"device_info"` // Crucial pour l'UI (ex: "iPhone 15", "Chrome sur Windows")
	IPHistory  []string       `json:"ip_history"`
	CreatedAt  time.Time      `json:"created_at"`
	ExpiresAt  time.Time      `json:"expires_at"`
}
