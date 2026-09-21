package report_models

// ReportPayload est le paquet envoyé au Worker pour la BDD.
type ReportPayload struct {
	ID         int64   `json:"id"`
	ReporterID int64   `json:"reporter_id"`
	TargetType int     `json:"target_type"`
	TargetIDs  []int64 `json:"target_ids"`
	Category   int     `json:"category"`
	Reason     string  `json:"reason"`
	Rationale  string  `json:"rationale"`
	State      int     `json:"state"`
	Importance float64 `json:"importance"` // ✅ NOUVEAU : Valeur économique du signalement
	CreatedAt  int64   `json:"created_at"`
	UpdatedAt  int64   `json:"updated_at"`
}
