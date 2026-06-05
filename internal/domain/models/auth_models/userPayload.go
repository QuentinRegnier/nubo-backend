package auth_models

import "time"

type UserPayload struct { // CreateUser
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
