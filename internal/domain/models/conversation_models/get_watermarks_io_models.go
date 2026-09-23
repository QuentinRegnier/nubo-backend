package conversation_models

// GetWatermarksInput valide la requête POST entrante.
// Zéro paramètre d'URL : l'ID de la conversation est dans le JSON.
type GetWatermarksInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

// GetWatermarksOutput structure la réponse.
// On utilise une map avec des clés string car le standard JSON exige des clés sous forme de chaîne,
// même si ce sont des identifiants (user_id).
type GetWatermarksOutput struct {
	Watermarks map[string]int64 `json:"watermarks"`
}
