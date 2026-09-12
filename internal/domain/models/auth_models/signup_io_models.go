package auth_models

import (
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
)

type SignUpInput struct {
	Username               string                                         `json:"username" binding:"required,min=3,max=30,alphanum" example:"johndoe"`
	Email                  string                                         `json:"email" binding:"required,email,max=100" example:"john@nubo.com"`
	Phone                  string                                         `json:"phone" binding:"required,e164" example:"+33612345678"`
	PasswordHash           string                                         `json:"password_hash" binding:"required,min=8" example:"secretPass123"`
	FirstName              string                                         `json:"first_name" binding:"max=50" example:"John"`
	LastName               string                                         `json:"last_name" binding:"max=50" example:"Doe"`
	Birthdate              string                                         `json:"birthdate" binding:"required,len=8,numeric" example:"25121990"`
	Gender                 int                                            `json:"gender" binding:"omitempty,oneof=0 1 2" example:"1"`
	Bio                    string                                         `json:"bio" binding:"max=500" example:"J'aime la tech"`
	Location               string                                         `json:"location" binding:"max=100" example:"Paris"`
	School                 string                                         `json:"school" binding:"max=100" example:"42"`
	Work                   string                                         `json:"work" binding:"max=100" example:"Developer"`
	DeviceInfo             map[string]any                                 `json:"device_info" example:"{\"model\":\"iphone\",\"os\":\"ios15\"}"`
	FirebaseInstallationID string                                         `json:"firebase_installation_id" binding:"required" example:"eyJhbGciOiJIUzI1Ni..."`
	DisplayAndContent      user_settings_models.DisplayAndContentSettings `json:"display_and_content" binding:"omitempty"`
	Privacy                user_settings_models.PrivacySettings           `json:"privacy" binding:"omitempty"`
	Notifications          user_settings_models.NotificationSettings      `json:"notifications" binding:"omitempty"`
	ProfilePictureID       int64                                          `json:"profile_picture_id" binding:"omitempty"` // <-- NOUVEAU
}

type SignUpResponse struct {
	UserID             int64                  `json:"user_id" example:"42"`
	MasterToken        string                 `json:"master_token" example:"eyJhbGciOiJIUzI1Ni..."`
	JWT                string                 `json:"jwt" example:"eyJhbGciOiJIUzI1Ni..."`
	ExpiresAt          time.Time              `json:"expires_at"`
	Message            string                 `json:"message" example:"User created successfully"`
	Avatar             media_models.MediaView `json:"avatar"` // <-- REMPLACE ProfilePictureID
	TelemetryVector    []float32              `json:"telemetry_vector"`
	TelemetryTopTags   []string               `json:"telemetry_top_tags"`
	TelemetryTimestamp int64                  `json:"telemetry_timestamp"`
}
