package relation_models

type GetBlockedInput struct {
	Limit  int64 `json:"limit" binding:"omitempty,min=1,max=100"`
	Offset int64 `json:"offset" binding:"omitempty,min=0"`
}

type GetBlockedOutput struct {
	Users []RelationUserView `json:"users"`
}
