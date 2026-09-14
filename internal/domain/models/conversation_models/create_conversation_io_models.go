package conversation_models

import "time"

type CreateConversationInput struct {
	Type           int                  `json:"type" binding:"required,oneof=0 1 2 3"`
	Title          string               `json:"title" binding:"omitempty,max=100"`
	Settings       ConversationSettings `json:"settings"` // Remplacement de Laws
	ParticipantIDs []int64              `json:"participant_ids" binding:"required,min=1,max=100"`
}

type CreateConversationOutput struct {
	ConversationID  int64     `json:"conversation_id"`
	AddedUserIDs    []int64   `json:"added_user_ids,omitempty"`    // Membres ajoutés automatiquement (pour les groupes)
	InvitedUserIDs  []int64   `json:"invited_user_ids,omitempty"`  // Membres invités (pour les groupes)
	RejectedUserIDs []int64   `json:"rejected_user_ids,omitempty"` // Membres refusés (pour les groupes)
	MessageIDs      []int64   `json:"message_ids,omitempty"`       // IDs des messages d'invitation générés (pour les groupes)
	ConversationIDs []int64   `json:"conversation_ids,omitempty"`  // IDs des conversations MP correspondantes (pour les groupes)
	InboxUpdateAt   time.Time `json:"inbox_update_at"`
}
