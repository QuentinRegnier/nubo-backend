package sync_models

type SyncPresenceInput struct {
	UserIDs []int64 `json:"user_ids" binding:"required,max=50"` // Max 50 pour éviter le spam réseau
}
