package post_models

import (
	"github.com/QuentinRegnier/numan-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/models/media_models"
)

type GetPostInput struct {
	PostIDs []int64 `json:"post_ids" binding:"required,min=1,max=50"`
}

type GetPostOutput struct {
	PostID         int64                             `json:"post_id"`
	Data           PostPayload                       `json:"data,omitempty"`
	AuthorUsername string                            `json:"author_username,omitempty"` // ✅ NOUVEAU
	AuthorAvatar   media_models.MediaView            `json:"author_avatar,omitempty"`   // ✅ NOUVEAU
	Media          []media_models.MediaView          `json:"media,omitempty"`
	Comments       []comment_models.GetCommentOutput `json:"comments,omitempty"`
	Error          string                            `json:"numan_error,omitempty"`
}
