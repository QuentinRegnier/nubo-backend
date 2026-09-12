package conversation_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UpdateConversation gère la modification (Titre, Règles, Description, Avatar) d'un groupe ou d'une communauté.
func UpdateConversation(ctx context.Context, callerID int64, convID int64, input conversation_models.UpdateConversationInput) error {
	// 1. VÉRIFICATION SÉCURITÉ ET RÉCUPÉRATION (Objet Complet)
	conv, err := security_service.LeftConversation(ctx, convID, callerID)
	if err != nil {
		return err // L'erreur est déjà formatée par LeftConversation
	}

	// 2. RÈGLES MÉTIER
	if conv.Type == 0 {
		return nubo_error.NewForbidden("INVALID_CONV_TYPE", "Impossible de modifier les métadonnées d'un message privé.", nil)
	}

	// 3. APPLICATION DES MODIFICATIONS
	isTitleUpdated := false
	if input.Title != conv.Title {
		conv.Title = input.Title
		isTitleUpdated = true
	}

	isDescUpdated := false
	isAvatarUpdated := false

	// Vérification de l'exclusivité des fonctionnalités de Communauté Publique (Type 3)
	if conv.Type == 3 {
		if input.Description != conv.Description {
			conv.Description = input.Description
			isDescUpdated = true
		}
		if input.AvatarID != conv.AvatarID {
			conv.AvatarID = input.AvatarID
			isAvatarUpdated = true
		}
	} else {
		// Bouclier : on rejette la requête avec un 403 Forbidden s'ils essaient d'injecter une description ou un avatar sur un groupe classique
		if (input.Description != "" && input.Description != conv.Description) || (input.AvatarID != 0 && input.AvatarID != conv.AvatarID) {
			return nubo_error.NewForbidden("FEATURE_NOT_SUPPORTED", "Seules les communautés publiques peuvent posséder une description et un avatar.", nil)
		}
	}

	conv.Laws = input.Laws
	conv.UpdatedAt = time.Now().UTC()

	// === MISE À JOUR IMMÉDIATE DU L1 (Object Cache) ===
	_ = object_cache_service.SetConversationInObjectCache(ctx, conv)

	// 4. DÉLÉGATION À LA FILE ASYNCHRONE (Write-Behind)
	// PartitionKey = convID pour garantir que cet Update passe après la Création dans la BDD
	err = redis.EnqueueDB(ctx, conv.ID, conv.ID, redis.EntityConversation, redis.ActionUpdate, conv, redis.TargetAll)
	if err != nil {
		return err
	}

	// 5. MISE À JOUR IMMÉDIATE DU L1 (Uniquement si une metadata visuelle a changé pour l'UI)
	if isTitleUpdated || isDescUpdated || isAvatarUpdated {
		convLite := lite_models.ConvLiteRequest{
			ID:            conv.ID,
			Type:          conv.Type,
			Title:         conv.Title,
			Description:   conv.Description, // NOUVEAU
			AvatarID:      conv.AvatarID,    // NOUVEAU
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
