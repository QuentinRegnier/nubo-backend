package media_models

// MediaView représente un média avec son ID (pour le cache local) et son URL signée prête à l'emploi.
type MediaView struct {
	MediaID int64  `json:"media_id"`
	URL     string `json:"url"`
}
