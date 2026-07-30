package telemetry_models

// SyncInput est la structure interne passée au service
type SyncInput struct {
	UserID  int64       `json:"-"`
	Payload SyncPayload `json:"payload"`
}

// SyncOutput est la réponse renvoyée au client (gère la bidirectionnalité)
type SyncOutput struct {
	Status        string    `json:"status"`
	NeedUpdate    bool      `json:"need_update"`              // Indique au client s'il doit écraser sa mémoire locale
	ServerVector  []float32 `json:"server_vector,omitempty"`  // Renvoyé uniquement si NeedUpdate = true
	ServerTags    []string  `json:"server_tags,omitempty"`    // Renvoyé uniquement si NeedUpdate = true
	ServerUpdated int64     `json:"server_updated,omitempty"` // Timestamp absolu du serveur
}
