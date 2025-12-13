package tools

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
	ID               int       `bson:"id" json:"id"` // Attention: défini comme Int dans ton schéma
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
	ProfilePictureID int       `bson:"profile_picture_id" json:"profile_picture_id"`
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
	ID           int            `bson:"id" json:"id"`
	UserID       int            `bson:"user_id" json:"user_id"`
	RefreshToken string         `bson:"refresh_token" json:"refresh_token"`
	DeviceToken  string         `bson:"device_token" json:"device_token"` // FCM token
	DeviceInfo   map[string]any `bson:"device_info" json:"device_info"`   // JSONB
	IPHistory    []string       `bson:"ip_history" json:"ip_history"`     // INET[]
	CreatedAt    time.Time      `bson:"created_at" json:"created_at"`
	ExpiresAt    time.Time      `bson:"expires_at" json:"expires_at"`
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
	DeviceToken  string         `json:"device_token" binding:"required" example:"device_123"`
	DeviceInfo   map[string]any `json:"device_info" example:"{\"model\":\"iphone\",\"os\":\"ios15\"}"`
}
type SignUpResponse struct {
	UserID           int       `json:"user_id" example:"42"`
	Token            string    `json:"token" example:"eyJhbGciOiJIUzI1Ni..."`
	ExpiresAt        time.Time `json:"expires_at"`
	Message          string    `json:"message" example:"User created successfully"`
	ProfilePictureID int       `bson:"profile_picture_id" json:"profile_picture_id"`
}
type LoginInput struct {
	Email        string   `json:"email" binding:"required,email" example:"john@nubo.com"`
	PasswordHash string   `json:"password_hash" binding:"required" example:"hashed_secret_123"`
	DeviceToken  string   `json:"device_token" binding:"required" example:"device_token_xyz"`
	DeviceInfo   []string `json:"device_info" example:"iphone,ios15"`
}
type LoginResponse struct {
	UserID        int       `json:"user_id" example:"42"`
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
	Token         string    `json:"token" example:"eyJhbGci..."`
	ExpiresAt     time.Time `json:"expires_at"`
	Message       string    `json:"message" example:"Login successful"`
}
