package post_models

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
)

type GetPostInput struct {
	UserID  int64   `json:"-"` // Protégé par JWT
	PostIDs []int64 `json:"post_ids" binding:"required,min=1,max=50"`
}

type GetPostOutput struct {
	PostID   int64                             `json:"post_id"`
	Data     PostPayload                       `json:"data,omitempty"`
	Media    []media_models.MediaView          `json:"media,omitempty"` // <-- MODIFIÉ
	Comments []comment_models.GetCommentOutput `json:"comments,omitempty"`
	Error    string                            `json:"nubo_error,omitempty"`
}
