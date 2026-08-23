package conversation_models

// AddMemberInput valide la demande d'ajout de membres dans un groupe
type AddMemberInput struct {
	ConversationID int64   `json:"conversation_id" binding:"required"`
	ParticipantIDs []int64 `json:"participant_ids" binding:"required,min=1,max=100"`
}

type AddMemberOutput struct {
	AddedUserIDs    []int64 `json:"added_user_ids"`    // Membres ajoutés automatiquement
	InvitedUserIDs  []int64 `json:"invited_user_ids"`  // Membres à qui une invitation a été envoyée
	RejectedUserIDs []int64 `json:"rejected_user_ids"` // Membres refusés par manque de droits de communication
	MessageIDs      []int64 `json:"message_ids"`       // IDs des messages d'invitation générés (vide si aucun)
	ConversationIDs []int64 `json:"conversation_ids"`  // IDs des conversations MP correspondantes (vide si aucun)
}
