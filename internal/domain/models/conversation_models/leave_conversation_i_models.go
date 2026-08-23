package conversation_models

// LeaveConversationInput contient les données nécessaires pour quitter ou supprimer une conversation
type LeaveConversationInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	NewOwnerID     int64 `json:"new_owner_id" binding:"omitempty"`
}
