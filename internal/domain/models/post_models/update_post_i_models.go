package post_models

type UpdatePostInput struct {
	PostID      int64    `json:"post_id" binding:"required"` // Obligatoire dans le corps de la requête
	Content     string   `json:"content" binding:"max=2200"`
	Hashtags    []string `json:"hashtags" binding:"max=10,dive,alphanum,max=50"`
	Identifiers []int64  `json:"identifiers" binding:"max=10"`
	Location    string   `json:"location" binding:"max=100"`
	Visibility  int      `json:"visibility" binding:"oneof=0 1 2"`
}
