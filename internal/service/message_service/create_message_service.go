package message_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/worker"
)

// CreateMessage construit le message et active les médias orphelins.
func CreateMessage(ctx context.Context, senderID int64, convID int64, input message_models.CreateMessageInput, isInternal bool) (int64, error) {
	// 1. SÉCURITÉ DES TYPES DE MESSAGES (Filtre anti-usurpation)
	if !isInternal {
		switch input.MessageType {
		case 0, 2, 3: // Texte, Image, GIF : OK
		case 1, 4: // Vocales, Vidéos : En attente d'implémentation
			return 0, errors.New("action refusée : les messages vocaux et vidéos ne sont pas encore supportés")
		case 5, 6, 7, 8: // Systèmes, Invitations, Liens
			return 0, errors.New("action refusée : vous n'avez pas l'autorisation d'envoyer ce type de message")
		default:
			return 0, errors.New("type de message inconnu")
		}
	}

	// 2. SÉCURITÉ : L'utilisateur doit être membre actif
	mem, err := security_service.LeftMember(ctx, convID, senderID)
	if err != nil {
		return 0, err
	}
	if mem.Role < 0 {
		return 0, errors.New("accès refusé : vous êtes banni de cette conversation")
	}

	// 3. ACTIVATION DU MÉDIA (OUT-OF-BAND)
	attachMap := input.Attachments
	if attachMap == nil {
		attachMap = make(map[string]any)
	}

	if input.MessageType == 2 {
		// A. On extrait le media_id fourni par le client
		rawMediaID, exists := attachMap["media_id"]
		if !exists {
			return 0, errors.New("media_id manquant pour un message de type image")
		}

		var mediaID int64
		switch v := rawMediaID.(type) {
		case float64:
			mediaID = int64(v)
		case int64:
			mediaID = v
		}

		// B. On le récupère instantanément en RAM (O(1))
		mediaPayload, errMedia := object_cache_service.GetMediaFromObjectCache(ctx, mediaID)
		if errMedia != nil {
			return 0, errors.New("média introuvable ou expiré")
		}

		// C. Sécurité Zero-Trust : Est-ce bien son image ?
		if mediaPayload.OwnerID != senderID {
			return 0, errors.New("accès refusé : ce média ne vous appartient pas")
		}

		// D. ACTIVATION ! On sort l'image de son état orphelin
		mediaPayload.Visibility = true
		mediaPayload.UpdatedAt = time.Now().UTC()

		// E. Sauvegarde L1 et Write-Behind L2/L3
		_ = object_cache_service.SetMediaInObjectCache(ctx, mediaPayload)
		err = redis.EnqueueDB(ctx, mediaID, senderID, redis.EntityMedia, redis.ActionUpdate, mediaPayload, redis.TargetAll)
		if err != nil {
			return 0, fmt.Errorf("erreur lors de la validation du média: %v", err)
		}
	} else {
		// Prévention stricte des "Messages Fantômes" (Texte vide)
		if input.Content == "" && input.MessageType == 0 {
			return 0, errors.New("le message ne peut pas être vide")
		}
	}

	// 4. PRÉPARATION DU MESSAGE
	msgID := pkg.GenerateID()
	now := time.Now().UTC()

	msgPayload := message_models.MessagePayload{
		ID:             msgID,
		ConversationID: convID,
		SenderID:       senderID,
		MessageType:    input.MessageType,
		Visibility:     true,
		Content:        input.Content,
		Attachments:    attachMap,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 5. MISE EN CACHE L1 IMMÉDIATE (Object Cache LFU)
	_ = object_cache_service.SetMessageInObjectCache(ctx, msgPayload)
	destinataires, _ := cache_service.ProcessNewMessageInSpeedCache(ctx, msgID, convID, senderID)
	conv, _ := object_cache_service.GetConversationFromObjectCache(ctx, convID)

	// 6. DISTRIBUTION TEMPS RÉEL (WebSockets)
	if conv.Type == 2 || conv.Type == 3 {
		_ = realtime_service.DistributeToCommunity(ctx, "message.created", msgPayload, convID)
	} else {
		_ = realtime_service.DistributeToUsers(ctx, "message.created", msgPayload, destinataires)
	}

	// 7. BATCHING DES COMPTEURS BDD
	for _, uID := range destinataires {
		worker.RegisterUnread(convID, uID)
	}

	// 8. ENVOI À LA FILE ASYNCHRONE (Write-Behind)
	err = redis.EnqueueDB(ctx, msgID, convID, redis.EntityMessage, redis.ActionCreate, msgPayload, redis.TargetAll)
	if err != nil {
		return 0, err
	}

	return msgID, nil
}
