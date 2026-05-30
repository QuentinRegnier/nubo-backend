package auth_models

import "time"

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
