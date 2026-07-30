package auth_models

// UpdateProfileInput définit les champs publics modifiables.
// Les pointeurs (*string) permettent de différencier un champ non envoyé d'un champ vide "".
type UpdateProfileInput struct {
	Username  *string `json:"username" binding:"omitempty,min=3,max=30,alphanum"`
	Email     *string `json:"email" binding:"omitempty,email,max=100"`
	Phone     *string `json:"phone" binding:"omitempty,e164"`
	FirstName *string `json:"first_name" binding:"omitempty,max=50"`
	LastName  *string `json:"last_name" binding:"omitempty,max=50"`
	Bio       *string `json:"bio" binding:"omitempty,max=500"`
	Location  *string `json:"location" binding:"omitempty,max=100"`
	School    *string `json:"school" binding:"omitempty,max=100"`
	Work      *string `json:"work" binding:"omitempty,max=100"`
}
