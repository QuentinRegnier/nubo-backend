package relation_models

import "time"

type RelationPayload struct {
	ID          int64     `json:"id" bson:"id"`
	PrimaryID   int64     `json:"primary_id" bson:"primary_id"`
	SecondaryID int64     `json:"secondary_id" bson:"secondary_id"`
	State       int       `json:"state" bson:"state"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
}
