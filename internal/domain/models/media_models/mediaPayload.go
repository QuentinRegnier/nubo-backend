package media_models

// MediaPayload représente l'entité d'un fichier média (image, vidéo) dans Nubo.
type MediaPayload struct {
	ID          int64  `json:"id" bson:"id"`
	OwnerID     int64  `json:"owner_id" bson:"owner_id"`
	StoragePath string `json:"storage_path" bson:"storage_path"`
	// Visibility agit d'abord comme un "Statut" lors d'un upload Out-of-Band :
	// false = pending (coquille vide en attente de la confirmation WebSocket)
	// true  = validé (le média est attaché à un message/post et est publiquement visible)
	Visibility bool  `json:"visibility" bson:"visibility"`
	CreatedAt  int64 `json:"created_at" bson:"created_at"`
	UpdatedAt  int64 `json:"updated_at" bson:"updated_at"`
}
