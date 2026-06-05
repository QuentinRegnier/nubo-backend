package report_models

import "time"

// ReportPayload est le paquet envoyé au Worker pour la BDD.
type ReportPayload struct {
	ID         int64     `json:"id"`
	ReporterID int64     `json:"reporter_id"`
	TargetType int       `json:"target_type"`
	TargetIDs  []int64   `json:"target_ids"`
	Category   int       `json:"category"`
	Reason     string    `json:"reason"`
	State      int       `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
