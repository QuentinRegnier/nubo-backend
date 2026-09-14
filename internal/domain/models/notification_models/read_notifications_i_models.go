package notification_models

import "time"

// ReadNotificationsInput valide l'ID Snowflake jusqu'où l'utilisateur a lu ses notifications
type ReadNotificationsInput struct {
	ReadUpToID int64 `json:"read_up_to_id" binding:"required"`
}

type ReadNotificationsOutput struct {
	ActivityUpdateAt time.Time `json:"activity_update_at"`
}
