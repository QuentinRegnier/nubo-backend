package message_models

// ReactMessageInput valide l'alias de la réaction.
type ReactMessageInput struct {
	MessageID int64  `json:"message_id" binding:"required"`
	Reaction  string `json:"reaction" binding:"required,min=1,max=30,print"` // ex: "heart" - Rejette les chaînes vides
}
