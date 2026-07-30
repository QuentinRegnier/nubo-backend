package relation_models

type RelationActionInput struct {
	TargetID int64 `json:"target_id" binding:"required"`
}
