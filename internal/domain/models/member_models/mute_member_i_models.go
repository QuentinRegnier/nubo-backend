package member_models

// MuteMemberInput valide la requête pour muter ou démuter un participant.
// Aucun paramètre dans l'URL, tout est ici.
type MuteMemberInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	TargetUserID   int64 `json:"target_user_id" binding:"required"`
	// Timestamp Unix en millisecondes. Si 0 = Démute immédiat.
	RestrictedUntil int64 `json:"restricted_until" binding:"min=0"`
}
