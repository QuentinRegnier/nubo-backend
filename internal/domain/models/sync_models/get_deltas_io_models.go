package sync_models

// GetDeltasInput valide la requête POST entrante.
// Zéro paramètre d'URL : le timestamp est dans le JSON.
type GetDeltasInput struct {
	SinceMs int64 `json:"since_ms" binding:"required,min=0"`
}

// GetDeltasOutput structure la réponse.
type GetDeltasOutput struct {
	ModifiedConversationIDs []int64 `json:"modified_conversation_ids"`
}
