package auth_models

import "time"

type LoginInput struct {
	Email                  string         `json:"email" binding:"required,email" example:"john@nubo.com"`
	PasswordHash           string         `json:"password_hash" binding:"required" example:"hashed_secret_123"`
	DeviceInfo             map[string]any `json:"device_info" example:"{\"os\":\"ios\",\"model\":\"iphone\"}"`
	FirebaseInstallationID string         `json:"firebase_installation_id" binding:"required" example:"eyJhbGciOiJIUzI1Ni..."`
}

// LoginResponse est désormais allégée à l'extrême (Le reste sera géré par /sync/identity)
type LoginResponse struct {
	UserID      int64     `json:"user_id" example:"42"`
	MasterToken string    `json:"master_token"`
	JWT         string    `json:"jwt" example:"eyJhbGciOiJIUzI1Ni..."`
	ExpiresAt   time.Time `json:"expires_at"`
	Message     string    `json:"message" example:"Login successful"`
}
