package notification_models

// SyncActivityInput récupère l'état actuel du cache du téléphone de l'utilisateur.
type SyncActivityInput struct {
	ClientUpdatedAt int64 `json:"client_updated_at" binding:"min=0"`
	ReadUpToID      int64 `json:"read_up_to_id" binding:"min=0"` // Le Snowflake ID de la notif la plus récente que le client possède
}

// SyncActivityOutput ne renvoie les notifications que si elles sont nouvelles.
type SyncActivityOutput struct {
	NeedUpdate    bool                  `json:"need_update"`
	ServerUpdated int64                 `json:"server_updated"`
	Notifications []NotificationPayload `json:"notifications,omitempty"`
}
