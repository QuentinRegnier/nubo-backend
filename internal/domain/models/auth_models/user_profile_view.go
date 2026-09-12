package auth_models

import "time"

// UserProfileView est le DTO sécurisé envoyé au client.
// Il exclut physiquement toutes les données de modération et les secrets.
type UserProfileView struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Birthdate time.Time `json:"birthdate"`
	Sex       int       `json:"sex"`
	Bio       string    `json:"bio"`
	Grade     int       `json:"grade"`
	Location  string    `json:"location"`
	School    string    `json:"school"`
	Work      string    `json:"work"`
	Badges    []string  `json:"badges"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsOnline  bool      `json:"is_online"` // NOUVEAU : Statut de présence
}
