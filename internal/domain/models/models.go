package models

import "time"

//TODO Séparer tous en différent fichier pour plus de clareté

// ********************************************************
// ****             Type in Intern                     ****
// ********************************************************

type Phone struct {
	CountryCode int `json:"country_code" binding:"required"` // ex: 33
	Number      int `json:"number" binding:"required"`       // ex: 746294017
}
type Location struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}
type SessionsRequest struct { // CreateSession
	ID                     int64          `bson:"id" json:"id"`
	UserID                 int64          `bson:"user_id" json:"user_id"`
	MasterToken            string         `bson:"master_token" json:"master_token"`
	FirebaseInstallationID string         `bson:"firebase_installation_id" json:"firebase_installation_id"`
	DeviceInfo             map[string]any `bson:"device_info" json:"device_info"`
	IPHistory              []string       `bson:"ip_history" json:"ip_history"`
	CurrentSecret          string         `bson:"current_secret" json:"current_secret"`
	LastSecret             string         `bson:"last_secret" json:"last_secret"`
	LastJWT                string         `bson:"last_jwt" json:"last_jwt"`
	ToleranceTime          time.Time      `bson:"tolerance_time" json:"tolerance_time"`
	CreatedAt              time.Time      `bson:"created_at" json:"created_at"`
	ExpiresAt              time.Time      `bson:"expires_at" json:"expires_at"`
}

type MediaRequest struct {
	ID          int64  `bson:"id" json:"id"`
	OwnerID     int64  `bson:"owner_id" json:"owner_id"`
	StoragePath string `bson:"storage_path" json:"storage_path"`
	// Visibility agit d'abord comme un "Statut" lors d'un upload Out-of-Band :
	// false = pending (coquille vide en attente de la confirmation WebSocket)
	// true  = validé (le média est attaché à un message/post et est publiquement visible)
	Visibility bool      `bson:"visibility" json:"visibility"`
	CreatedAt  time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at" json:"updated_at"`
}

// ********************************************************
// ****           Speed Cache Types (Lite)             ****
// ********************************************************

type UserLiteRequest struct {
	ID                     int64    `bson:"id" json:"id"`
	Username               string   `bson:"username" json:"username"`
	FirstName              string   `bson:"first_name" json:"first_name"`
	LastName               string   `bson:"last_name" json:"last_name"`
	ProfilePictureID       int64    `bson:"profile_picture_id" json:"profile_picture_id"`
	Bio                    string   `bson:"bio" json:"bio"`
	Grade                  int      `bson:"grade" json:"grade"`
	Badges                 []string `bson:"badges" json:"badges"`
	ConversationPermission int      `bson:"conversation_permission" json:"conversation_permission"` // 0=Tout le monde, 1=Abonnés, 2=Amis
	AddGroupPermission     bool     `bson:"add_group_permission" json:"add_group_permission"`       // true=Auto, false=Invitation
}

type ConvLiteRequest struct {
	ID            int64  `bson:"id" json:"id"`
	Type          int    `bson:"type" json:"type"`
	Title         string `bson:"title" json:"title"`
	LastMessageID int64  `bson:"last_message_id" json:"last_message_id"`
}

type MemberLiteRequest struct {
	ConversationID  int64 `bson:"conversation_id" json:"conversation_id"`
	UserID          int64 `bson:"user_id" json:"user_id"`
	UnreadCount     int   `bson:"unread_count" json:"unread_count"`
	Role            int   `bson:"role" json:"role"`
	FrozenMessageID int64 `bson:"frozen_message_id" json:"frozen_message_id"`
	JoinedAt        int64 `bson:"joined_at" json:"joined_at"` // AJOUT POUR LE TRI
}
