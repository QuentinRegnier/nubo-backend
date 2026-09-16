package auth_models

type SessionsPayload struct { // CreateSession
	ID                     int64          `bson:"id" json:"id"`
	UserID                 int64          `bson:"user_id" json:"user_id"`
	MasterToken            string         `bson:"master_token" json:"master_token"`
	FirebaseInstallationID string         `bson:"firebase_installation_id" json:"firebase_installation_id"`
	DeviceInfo             map[string]any `bson:"device_info" json:"device_info"`
	IPHistory              []string       `bson:"ip_history" json:"ip_history"`
	CurrentSecret          string         `bson:"current_secret" json:"current_secret"`
	LastSecret             string         `bson:"last_secret" json:"last_secret"`
	LastJWT                string         `bson:"last_jwt" json:"last_jwt"`
	ToleranceTime          int64          `bson:"tolerance_time" json:"tolerance_time"`
	CreatedAt              int64          `bson:"created_at" json:"created_at"`
	ExpiresAt              int64          `bson:"expires_at" json:"expires_at"`
}
