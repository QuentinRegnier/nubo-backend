package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoGetMember récupère le payload complet d'un membre avec le Smart Fallback (Retard BDD)
func MongoGetMember(convID int64, userID int64) (conversation_models.MemberPayload, error) {
	var mem conversation_models.MemberPayload

	filter := map[string]any{"conversation_id": convID, "user_id": userID}
	docs, err := Members.Get(filter, nil)
	if err != nil || len(docs) == 0 {
		return mem, nubo_error.NewNotFound("MEMBER_NOT_FOUND", "Membre introuvable.", err)
	}

	if err := pkg.ToStruct(docs[0], &mem); err != nil {
		return mem, nubo_error.NewInternal(fmt.Errorf("conversion document to struct: %w", err))
	}

	// SMART RE-COUNT : Si le compteur est à 0 ou qu'on a plus de 5s de retard
	if mem.UnreadCount == 0 || time.Since(mem.UpdatedAt) > 5*time.Second {
		countFilter := map[string]any{
			"conversation_id": convID,
			"sender_id":       map[string]any{"$ne": userID},
			"created_at":      map[string]any{"$gt": mem.UpdatedAt}, // Strictement après la dernière sauvegarde
		}

		// Un simple CountDocuments natif (très rapide en MongoDB)
		realUnread, errCount := Messages.DB.Collection(Messages.Name).CountDocuments(context.Background(), countFilter)
		if errCount == nil {
			// On ADDITIONNE la valeur en base avec les messages orphelins (modification RAM uniquement)
			mem.UnreadCount += int(realUnread)
		}
	}

	return mem, nil
}
