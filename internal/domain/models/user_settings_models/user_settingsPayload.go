package user_settings_models

import "time"

type UserSettingsPayload struct {
	ID            int64          `json:"id" bson:"id"`
	UserID        int64          `json:"user_id" bson:"user_id"`
	Privacy       map[string]any `json:"privacy" bson:"privacy"`
	Notifications map[string]any `json:"notifications" bson:"notifications"`
	Language      string         `json:"language" bson:"language"`
	Theme         int            `json:"theme" bson:"theme"`

	// --- NOUVEAU : Persistance du Vecteur Edge-to-Cloud ---
	TelemetryVector    []float32 `json:"telemetry_vector" bson:"telemetry_vector"`
	TelemetryTags      []string  `json:"telemetry_tags" bson:"telemetry_tags"`
	TelemetryTimestamp int64     `json:"telemetry_timestamp" bson:"telemetry_timestamp"`

	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}
