package post_models

type CreatePostInput struct {
	Content string `json:"content" binding:"max=2200"`
	// max=10 (pas plus de 10 tags), dive (applique la règle à chaque élément), alphanum (pas de # inclus), max=50
	Hashtags    []string `json:"hashtags" binding:"max=10,dive,alphanum,max=50"`
	Identifiers []int64  `json:"identifiers" binding:"max=10"`
	Location    string   `json:"location" binding:"max=100"`
	Visibility  int      `json:"visibility" binding:"oneof=0 1 2"`
}
type CreatePostResponse struct {
	PostID int64 `json:"post_id"`
}
