package auth_models

import "time"

type SignUpInput struct {
	// min=3, max=30, alphanum (pas d'espaces ou de caractères spéciaux)
	Username string `json:"username" binding:"required,min=3,max=30,alphanum" example:"johndoe"`
	Email    string `json:"email" binding:"required,email,max=100" example:"john@nubo.com"`
	// e164 garantit le format international (+33612345678)
	Phone        string         `json:"phone" binding:"required,e164" example:"+33612345678"`
	PasswordHash string         `json:"password_hash" binding:"required,min=8" example:"secretPass123"`
	FirstName    string         `json:"first_name" binding:"max=50" example:"John"`
	LastName     string         `json:"last_name" binding:"max=50" example:"Doe"`
	Birthdate    string         `json:"birthdate" binding:"required,len=8,numeric" example:"25121990"`
	Gender       *int           `json:"gender" binding:"omitempty,oneof=0 1 2" example:"1"`
	Bio          string         `json:"bio" binding:"max=500" example:"J'aime la tech"`
	Location     string         `json:"location" binding:"max=100" example:"Paris"`
	School       string         `json:"school" binding:"max=100" example:"42"`
	Work         string         `json:"work" binding:"max=100" example:"Developer"`
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
