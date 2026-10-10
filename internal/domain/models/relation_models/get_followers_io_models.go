package relation_models

type GetFollowersInput struct {
	TargetID int64 `json:"target_id"`
	Limit    int64 `json:"limit" binding:"omitempty,min=1,max=100"`
	Offset   int64 `json:"offset" binding:"omitempty,min=0"`
}

type GetFollowersOutput struct {
	Users []RelationUserView `json:"users"`
}
