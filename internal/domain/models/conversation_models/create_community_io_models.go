package conversation_models

// CreateCommunityInput valide les données pour la création d'une communauté publique (Type 3).
type CreateCommunityInput struct {
	Title   string `json:"title" binding:"required,min=3,max=100"`
	Laws    []int  `json:"laws" binding:"omitempty"`
	OwnerID int64  `json:"owner_id" binding:"omitempty"` // Exclusif aux Modérateurs et Admins
}

// CreateCommunityOutput renvoie l'ID généré.
type CreateCommunityOutput struct {
	ConversationID int64 `json:"conversation_id"`
}
