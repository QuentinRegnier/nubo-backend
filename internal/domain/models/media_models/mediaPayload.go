package media_models

import "time"

// MediaPayload représente l'entité d'un fichier média (image, vidéo) dans Nubo.
type MediaPayload struct {
	ID          int64     `json:"id" bson:"id"`
	OwnerID     int64     `json:"owner_id" bson:"owner_id"`
	StoragePath string    `json:"storage_path" bson:"storage_path"`
	Visibility  bool      `json:"visibility" bson:"visibility"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
}
