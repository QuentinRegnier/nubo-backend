package notification_models

// ReadNotificationsInput valide l'ID Snowflake jusqu'où l'utilisateur a lu ses notifications
type ReadNotificationsInput struct {
	ReadUpToID int64 `json:"read_up_to_id" binding:"required"`
}
