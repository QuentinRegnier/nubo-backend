package conversation_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UpdateConversation gère la modification (Titre, Règles) d'un groupe ou d'une communauté.
func UpdateConversation(ctx context.Context, callerID int64, convID int64, input conversation_models.UpdateConversationInput) error {
	// 1. VÉRIFICATION SÉCURITÉ ET RÉCUPÉRATION (Objet Complet)
	conv, err := security_service.LeftConversation(ctx, convID, callerID)
	if err != nil {
		return err
	}

	// 2. RÈGLES MÉTIER
	if conv.Type == 0 {
		return errors.New("impossible de modifier les métadonnées d'un message privé")
	}

	// 3. APPLICATION DES MODIFICATIONS
	isTitleUpdated := false
	if input.Title != conv.Title {
		conv.Title = input.Title
		isTitleUpdated = true
	}
	conv.Laws = input.Laws
	conv.UpdatedAt = time.Now().UTC()

	// === NOUVEAU : MISE À JOUR IMMÉDIATE DU L1 (Object Cache) ===
	_ = object_cache_service.SetConversationInObjectCache(ctx, conv)

	// 4. DÉLÉGATION À LA FILE ASYNCHRONE (Write-Behind)
	// PartitionKey = convID pour garantir que cet Update passe après la Création
	err = redis.EnqueueDB(ctx, conv.ID, conv.ID, redis.EntityConversation, redis.ActionUpdate, conv, redis.TargetAll)
	if err != nil {
		return err
	}

	// 5. MISE À JOUR IMMÉDIATE DU L1 (Uniquement si le titre a changé pour l'UI)
	if isTitleUpdated {
		convLite := models.ConvLiteRequest{
			ID:            conv.ID,
			Type:          conv.Type,
			Title:         conv.Title, // Sans pointeur
			LastMessageID: conv.LastMessageID,
		}
		_ = redis.ConvMeta.SetObject(ctx, conv.ID, convLite)
	}

	// 6. ENVOI NOTIFICATION (Asynchrone)
	go func() {
		err := realtime_service.BroadcastToConversation(context.Background(), conv.ID, "conversation.updated", conv)
		if err != nil {
			_ = fmt.Errorf("erreur lors de l'envoi de la notification de mise à jour de conversation : %v", err)
		}
	}()

	return nil
}
