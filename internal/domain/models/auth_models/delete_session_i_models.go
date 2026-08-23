package auth_models

type DeleteSessionInput struct {
	SessionID int64 `json:"session_id" binding:"required"`
}
