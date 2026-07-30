package telemetry_models

// TelemetryEvent représente un log d'interaction (aligné sur le TDD § 2.3)
type TelemetryEvent struct {
	PostID       int64 `json:"post_id"`
	DwellTimeMs  int   `json:"dwell_time_ms"`
	IsClicked    bool  `json:"is_clicked"`
	DeepScroll   bool  `json:"deep_scroll"`   // NOUVEAU: Scroll > 80% du post
	ProfileVisit bool  `json:"profile_visit"` // NOUVEAU: Clic sur l'avatar de l'auteur
}
