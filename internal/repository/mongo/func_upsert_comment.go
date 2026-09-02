package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoUpsertComment sauvegarde ou met à jour un commentaire en L2 (Promotion L3 -> L2)
func MongoUpsertComment(c comment_models.CommentPayload) error {
	doc, err := pkg.ToMap(c)
	if err != nil || doc == nil {
		return nubo_error.NewInternal(fmt.Errorf("erreur conversion commentaire mongo: %w", err))
	}
	// Upsert silencieux dans la collection
	return Comments.Set(doc)
}
