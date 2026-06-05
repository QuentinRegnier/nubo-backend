package report_models

// CreateReportInput valide la requête de l'utilisateur.
type CreateReportInput struct {
	UserID     int64   `json:"-"` // Protégé, injecté par le handler via JWT
	TargetType int     `json:"target_type" binding:"required,oneof=0 1 2 3 5"`
	TargetIDs  []int64 `json:"target_ids" binding:"required,min=1,max=20"` // Max 20 messages d'un coup
	Category   int     `json:"category" binding:"required"`
	Reason     string  `json:"reason" binding:"max=1000"` // Explications optionnelles de l'utilisateur
}
