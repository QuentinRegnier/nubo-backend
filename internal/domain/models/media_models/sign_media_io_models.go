package media_models

// SignMediaInput valide la demande de signature d'un média.
// L'application doit fournir l'ID du média ET le contexte (Post ou Conversation)
// dans lequel elle l'a vu, afin que le serveur puisse vérifier ses droits d'accès.
type SignMediaInput struct {
	MediaID        int64 `json:"media_id" binding:"required"`
	PostID         int64 `json:"post_id" binding:"omitempty"`
	ConversationID int64 `json:"conversation_id" binding:"omitempty"`
}

// SignMediaOutput renvoie l'URL finale signée cryptographiquement avec HMAC.
type SignMediaOutput struct {
	MediaID int64  `json:"media_id"`
	URL     string `json:"url"`
}
