package notification_models

// GetNotificationsInput valide les paramètres de la requête pour charger l'historique
type GetNotificationsInput struct {
	Limit  int64 `form:"limit,default=50" binding:"max=100"`
	Offset int64 `form:"offset,default=0" binding:"min=0"`
	Force  bool  `form:"force"` // Si true, force la purge du cache L1 et la lecture L2
}

type GetNotificationsOutput struct {
	Notifications []NotificationPayload `json:"notifications"`
}
