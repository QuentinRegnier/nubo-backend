package relation_models

type RelationPayload struct {
	ID          int64 `json:"id" bson:"id"`
	PrimaryID   int64 `json:"primary_id" bson:"primary_id"`
	SecondaryID int64 `json:"secondary_id" bson:"secondary_id"`
	State       int   `json:"state" bson:"state"`
	CreatedAt   int64 `json:"created_at" bson:"created_at"`
	UpdatedAt   int64 `json:"updated_at" bson:"updated_at"`
}
