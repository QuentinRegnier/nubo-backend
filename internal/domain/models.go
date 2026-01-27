package domain

import "time"

// Phone :
type Phone struct {
	CountryCode int `json:"country_code" binding:"required"` // ex: 33
	Number      int `json:"number" binding:"required"`       // ex: 746294017
}

// Location :
type Location struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

// RegisterRequest représente le payload complet envoyé par l'app
type UserRequest struct { // CreateUser
	ID               int64     `bson:"id" json:"id"` // Attention: défini comme Int dans ton schéma
	Username         string    `bson:"username" json:"username"`
	Email            string    `bson:"email" json:"email"`
	EmailVerified    bool      `bson:"email_verified" json:"email_verified"`
	Phone            string    `bson:"phone" json:"phone"`
	PhoneVerified    bool      `bson:"phone_verified" json:"phone_verified"`
	PasswordHash     string    `bson:"password_hash" json:"password_hash"`
	FirstName        string    `bson:"first_name" json:"first_name"`
	LastName         string    `bson:"last_name" json:"last_name"`
	Birthdate        time.Time `bson:"birthdate" json:"birthdate"` // reflect.Struct correspond souvent à time.Time
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

type RenewJWTResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}

// RefreshMasterInput : Structure du Body pour le hard-refresh
type RefreshMasterInput struct {
	UserID      int64  `json:"id_user" binding:"required"`
	MasterToken string `json:"master_token" binding:"required"` // L'ancien MasterToken
	Username    string `json:"username" binding:"required"`     // Le username de l'utilisateur
}

// RefreshMasterResponse : Ce qu'on renvoie au client
type RefreshMasterResponse struct {
	MasterToken string `json:"master_token"`
	Token       string `json:"token"` // Le nouveau JWT
	Message     string `json:"message"`
}

// Structure interne

type SignUpInput struct {
	Username     string         `json:"username" binding:"required" example:"johndoe"`
	Email        string         `json:"email" binding:"required,email" example:"john@nubo.com"`
	Phone        string         `json:"phone" binding:"required" example:"+33612345678"`
	PasswordHash string         `json:"password_hash" binding:"required" example:"secretPass123"`
	FirstName    string         `json:"first_name" example:"John"`
	LastName     string         `json:"last_name" example:"Doe"`
	Birthdate    string         `json:"birthdate" binding:"required,len=8" example:"25121990"` // ddmmaaaa
	Gender       *int           `json:"gender" example:"1"`                                    // 0, 1, 2
	Bio          string         `json:"bio" example:"J'aime la tech"`
	Location     string         `json:"location" example:"Paris"`
	School       string         `json:"school" example:"42"`
	Work         string         `json:"work" example:"Developer"`
	DeviceInfo   map[string]any `json:"device_info" example:"{\"model\":\"iphone\",\"os\":\"ios15\"}"`
	DeviceToken  string         `json:"device_token" binding:"required" example:"eyJhbGciOiJIUzI1Ni..."`
}
type SignUpResponse struct {
	UserID           int64     `json:"user_id" example:"42"`
	MasterToken      string    `json:"master_token" example:"eyJhbGciOiJIUzI1Ni..."`
	JWT              string    `json:"jwt" example:"eyJhbGciOiJIUzI1Ni..."`
	ExpiresAt        time.Time `json:"expires_at"`
	Message          string    `json:"message" example:"User created successfully"`
	ProfilePictureID int64     `bson:"profile_picture_id" json:"profile_picture_id"`
}
type LoginInput struct {
	Email        string         `json:"email" binding:"required,email" example:"john@nubo.com"`
	PasswordHash string         `json:"password_hash" binding:"required" example:"hashed_secret_123"`
	DeviceInfo   map[string]any `json:"device_info" example:"{\"os\":\"ios\",\"model\":\"iphone\"}"`
	DeviceToken  string         `json:"device_token" binding:"required" example:"eyJhbGciOiJIUzI1Ni..."`
}
type LoginResponse struct {
	UserID        int64     `json:"user_id" example:"42"`
	Username      string    `json:"username" example:"johndoe"`
	Email         string    `json:"email" example:"john@nubo.com"`
	EmailVerified bool      `json:"email_verified" example:"true"`
	Phone         string    `json:"phone" example:"+33612345678"`
	PhoneVerified bool      `json:"phone_verified" example:"true"`
	FirstName     string    `json:"first_name" example:"John"`
	LastName      string    `json:"last_name" example:"Doe"`
	Birthdate     time.Time `json:"birthdate"`
	Sex           int       `json:"sex" example:"1"`
	Bio           string    `json:"bio" example:"Bio..."`
	Grade         int       `json:"grade" example:"0"`
	Location      string    `json:"location" example:"Paris"`
	School        string    `json:"school" example:"42"`
	Work          string    `json:"work" example:"Dev"`
	Badges        []string  `json:"badges"`
	Desactivated  bool      `json:"desactivated" example:"false"`
	Banned        bool      `json:"banned" example:"false"`
	BanReason     string    `json:"ban_reason"`
	BanExpiresAt  time.Time `json:"ban_expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	MasterToken   string    `json:"master_token" example:"eyJhbGci..."`
	JWT           string    `json:"jwt" example:"eyJhbGciOiJIUzI1Ni..."`
	ExpiresAt     time.Time `json:"expires_at"`
	Message       string    `json:"message" example:"Login successful"`
}

type MediaRequest struct {
	ID          int64     `bson:"id" json:"id"`
	OwnerID     int64     `bson:"owner_id" json:"owner_id"`
	StoragePath string    `bson:"storage_path" json:"storage_path"`
	Visibility  bool      `bson:"visibility" json:"visibility"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}
