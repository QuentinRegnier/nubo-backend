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
type UserRequest struct { // CreateUser
	ID               int64     `bson:"id" json:"id"` // Attention : défini comme Int dans ton schéma
	Username         string    `bson:"username" json:"username"`
	Email            string    `bson:"email" json:"email"`
	EmailVerified    bool      `bson:"email_verified" json:"email_verified"`
	Phone            string    `bson:"phone" json:"phone"`
	PhoneVerified    bool      `bson:"phone_verified" json:"phone_verified"`
	PasswordHash     string    `bson:"password_hash" json:"password_hash"`
	FirstName        string    `bson:"first_name" json:"first_name"`
	LastName         string    `bson:"last_name" json:"last_name"`
	Birthdate        time.Time `bson:"birthdate" json:"birthdate"` // time.Time
	Sex              int       `bson:"sex" json:"sex"`
	Bio              string    `bson:"bio" json:"bio"`
	ProfilePictureID int64     `bson:"profile_picture_id" json:"profile_picture_id"`
	Grade            int       `bson:"grade" json:"grade"`
	Location         string    `bson:"location" json:"location"`
	School           string    `bson:"school" json:"school"`
	Work             string    `bson:"work" json:"work"`
	Badges           []string  `bson:"badges" json:"badges"` // reflect.Slice
	Desactivated     bool      `bson:"desactivated" json:"desactivated"`
	Banned           bool      `bson:"banned" json:"banned"`
	BanReason        string    `bson:"ban_reason" json:"ban_reason"`
	BanExpiresAt     time.Time `bson:"ban_expires_at" json:"ban_expires_at"`
	CreatedAt        time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time `bson:"updated_at" json:"updated_at"`
}
type SessionsRequest struct { // CreateSession
	ID            int64          `bson:"id" json:"id"`
	UserID        int64          `bson:"user_id" json:"user_id"`
	MasterToken   string         `bson:"master_token" json:"master_token"`
	DeviceToken   string         `bson:"device_token" json:"device_token"`
	DeviceInfo    map[string]any `bson:"device_info" json:"device_info"`
	IPHistory     []string       `bson:"ip_history" json:"ip_history"`
	CurrentSecret string         `bson:"current_secret" json:"current_secret"`
	LastSecret    string         `bson:"last_secret" json:"last_secret"`
	LastJWT       string         `bson:"last_jwt" json:"last_jwt"`
	ToleranceTime time.Time      `bson:"tolerance_time" json:"tolerance_time"`
	CreatedAt     time.Time      `bson:"created_at" json:"created_at"`
	ExpiresAt     time.Time      `bson:"expires_at" json:"expires_at"`
}

type MediaRequest struct {
	ID          int64     `bson:"id" json:"id"`
	OwnerID     int64     `bson:"owner_id" json:"owner_id"`
	StoragePath string    `bson:"storage_path" json:"storage_path"`
	Visibility  bool      `bson:"visibility" json:"visibility"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}

type PostRequest struct {
	ID            int64     `bson:"id" json:"id"`
	UserID        int64     `bson:"user_id" json:"user_id"`
	Content       string    `bson:"content" json:"content"`
	Hashtags      []string  `bson:"hashtags" json:"hashtags"`
	Identifiers   []int64   `bson:"identifiers" json:"identifiers"`
	MediaIDs      []int64   `bson:"media_ids" json:"media_ids"`
	Visibility    int       `bson:"visibility" json:"visibility"`
	Location      string    `bson:"location" json:"location"`
	CreatedAt     time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" msgpack:"updated_at" bson:"updated_at"`
	LikeCount     int       `json:"like_count" msgpack:"like_count" bson:"like_count"`
	CommentCount  int       `json:"comment_count" msgpack:"comment_count" bson:"comment_count"`
	ViewCount     int       `json:"view_count" msgpack:"view_count" bson:"view_count"`
	HasMedia      bool      `json:"has_media" msgpack:"has_media" bson:"has_media"`
	Vector        []float32 `json:"vector" msgpack:"vector" bson:"vector"`
	VectorVersion int       `json:"vector_version" msgpack:"vector_version" bson:"vector_version"`
}

// ********************************************************
// ****           Speed Cache Types (Lite)             ****
// ********************************************************

type UserLiteRequest struct {
	ID               int64    `bson:"id" json:"id"`
	Username         string   `bson:"username" json:"username"`
	FirstName        string   `bson:"first_name" json:"first_name"`
	LastName         string   `bson:"last_name" json:"last_name"`
	ProfilePictureID int64    `bson:"profile_picture_id" json:"profile_picture_id"`
	Bio              string   `bson:"bio" json:"bio"`
	Grade            int      `bson:"grade" json:"grade"`
	Badges           []string `bson:"badges" json:"badges"`
}

type ConvLiteRequest struct {
	ID            int64  `bson:"id" json:"id"`
	Type          int    `bson:"type" json:"type"`
	Title         string `bson:"title" json:"title"`
	LastMessageID int64  `bson:"last_message_id" json:"last_message_id"`
}

type MemberLiteRequest struct {
	ConversationID int64 `bson:"conversation_id" json:"conversation_id"`
	UserID         int64 `bson:"user_id" json:"user_id"`
	UnreadCount    int   `bson:"unread_count" json:"unread_count"`
	Role           int   `bson:"role" json:"role"`
}
