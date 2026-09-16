package auth_models

// UpdateProfileInput définit les champs publics modifiables (Remplacement intégral PUT).
type UpdateProfileInput struct {
	Username         string `json:"username" binding:"required,min=3,max=30,alphanum"`
	Email            string `json:"email" binding:"required,email,max=100"`
	Phone            string `json:"phone" binding:"omitempty,e164"` // omitempty car un format E164 vide crasherait
	FirstName        string `json:"first_name" binding:"required,max=50"`
	LastName         string `json:"last_name" binding:"required,max=50"`
	Bio              string `json:"bio" binding:"max=500"` // Sans omitempty : "" est valide
	Location         string `json:"location" binding:"max=100"`
	School           string `json:"school" binding:"max=100"`
	Work             string `json:"work" binding:"max=100"`
	ProfilePictureID int64  `json:"profile_picture_id" binding:"omitempty"`
}

type UpdateProfileOutput struct {
	ProfileUpdatedAt int64 `json:"profile_updated_at"`
}
