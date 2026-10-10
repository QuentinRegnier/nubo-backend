package relation_models

type GetFriendsInput struct {
	TargetID int64 `json:"target_id"`
	Limit    int64 `json:"limit" binding:"omitempty,min=1,max=100"`
	Offset   int64 `json:"offset" binding:"omitempty,min=0"`
}

type GetFriendsOutput struct {
	Users []RelationUserView `json:"users"`
}
