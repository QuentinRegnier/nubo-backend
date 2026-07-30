package telemetry_models

// SyncPayload correspond à la fusion TDD § 2.5 (Vecteur) + Télémétrie d'engagement
type SyncPayload struct {
	SchemaVersion int `json:"schema_version"`

	// Bloc 1 : Edge Vector (TDD § 2.5)
	Vector struct {
		Dims             int       `json:"dims"`
		EmbeddingVersion int       `json:"embedding_version"`
		Values           []float32 `json:"values"`
		NormPreUnit      float32   `json:"norm_pre_unit"`
	} `json:"vector"`

	// Bloc 2 : Méta-données et Confiance
	Meta struct {
		TotalInteractions int     `json:"total_interactions"`
		ConfidenceScore   float64 `json:"confidence_score"`
		ClientPlatform    string  `json:"client_platform"`
		ClientUpdatedAt   int64   `json:"client_updated_at"` // NOUVEAU: Timestamp de la dernière mise à jour locale
	} `json:"meta"`

	// Bloc 3 : Top Tags pour le Magasinier
	TopTags []string `json:"top_tags"`

	// Bloc 4 : Matrice d'Engagement (Batch)
	Telemetry []TelemetryEvent `json:"telemetry"`
}
