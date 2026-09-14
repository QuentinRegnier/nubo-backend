package message_service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// Structure locale privée pour casser la dépendance cyclique vers le package worker
type pushJob struct {
	UserID    int64  `json:"user_id"`
	EventType string `json:"event_type"`
	Payload   any    `json:"payload"`
}

// ShouldSendPush évalue si un utilisateur doit recevoir une notification Push
// en O(1) selon la matrice de présence et de confidentialité (DDD).
// Ajout de messageType dans la signature
func ShouldSendPush(ctx context.Context, convID int64, userID int64, isMentioned bool, messageType int) bool {
	// 1. Filtre de Présence (O(1))
	if cache_service.IsUserOnline(ctx, userID) {
		return false
	}

	// 2. Filtre Global L1 (O(1))
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if err != nil || !settings.Notifications.MasterPushEnabled {
		return false
	}

	// 3. Filtre Local L1 (O(1))
	member, err := object_cache_service.GetMemberFromObjectCache(ctx, convID, userID)
	if err != nil {
		return false
	}

	isMuted := member.Settings.IsMuted

	// Gestion du Mute Temporaire
	expireAt := member.Settings.MuteExpireAt
	if expireAt > 0 {
		now := time.Now().Unix()
		if expireAt > 9999999999 {
			now = time.Now().UnixMilli()
		}
		if expireAt > now {
			isMuted = 2
		}
	}

	notifyMentions := settings.Notifications.NotifyMentions

	// AIGUILLAGE MÉTIER : On utilise le réglage d'invitation si c'est un Type 6
	baseNotify := settings.Notifications.NotifyMessages
	if messageType == 6 {
		baseNotify = settings.Notifications.NotifyGroupInvites
	}

	// Matrice d'Entonnoir Décisionnelle
	switch isMuted {
	case 2:
		return false
	case 1:
		return isMentioned && notifyMentions
	case 0:
		// baseNotify vaut "NotifyMessages" (par défaut) ou "NotifyGroupInvites" (si Type 6)
		return (isMentioned && notifyMentions) || (!isMentioned && baseNotify)
	default:
		return false
	}
}

// dispatchPushNotifications itère sur les destinataires et route les jobs FCM
func dispatchPushNotifications(msgView message_models.MessageView, destinataires []int64, mentionedUserIDs []int64) {
	// Garde-fou : On ne notifie JAMAIS les messages systèmes (invitations, alertes) via le Push Messagerie
	if msgView.MessageType == 8 {
		return
	}

	// Exécution asynchrone pour garantir une latence HTTP < 5ms lors du CreateMessage
	go func() {
		bgCtx := context.Background()

		for _, destID := range destinataires {
			// On exclut catégoriquement l'expéditeur de la matrice
			if destID == msgView.SenderID {
				continue
			}

			isMentioned := pkg.Exists(mentionedUserIDs, destID)

			if ShouldSendPush(bgCtx, msgView.ConversationID, destID, isMentioned, msgView.MessageType) {
				eventType := "message.new"
				if isMentioned {
					eventType = "message.mention" // Catégorisation pour l'UI Flutter
				}

				// On utilise la structure locale pour éviter l'import cyclique
				job := pushJob{
					UserID:    destID,
					EventType: eventType,
					Payload:   msgView,
				}

				// Envoi dans la file FIFO du Worker Firebase via l'abstraction DDD
				if jobBytes, err := json.Marshal(job); err == nil {
					_ = redis.WorkerQueue.LPush(bgCtx, "firebase", jobBytes)
				}
			}
		}
	}()
}
