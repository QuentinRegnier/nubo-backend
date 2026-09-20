package relation_models

type GetBlockedInput struct {
	Limit  int `json:"limit" binding:"omitempty,min=1,max=100"`
	Offset int `json:"offset" binding:"omitempty,min=0"`
}

type GetBlockedOutput struct {
	Users []RelationUserView `json:"users"`
}
