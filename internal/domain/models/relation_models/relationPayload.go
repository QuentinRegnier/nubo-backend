package relation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"

type RelationPayload struct {
	ID          int64 `json:"id" bson:"id"`
	PrimaryID   int64 `json:"primary_id" bson:"primary_id"`
	SecondaryID int64 `json:"secondary_id" bson:"secondary_id"`
	State       int   `json:"state" bson:"state"`
	CreatedAt   int64 `json:"created_at" bson:"created_at"`
	UpdatedAt   int64 `json:"updated_at" bson:"updated_at"`
}

// RelationUserView enrichit le profil avec l'état de relation du point de vue de celui qui fait la requête
type RelationUserView struct {
	auth_models.UserLiteView
	ViewerRelationState int `json:"viewer_relation_state"` // 0=Rien, 1=Abonné, 2=Ami, -1=Bloqué
}
