package message_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/worker"
)

// CreateMessage construit le message et active les médias orphelins.
func CreateMessage(ctx context.Context, senderID int64, convID int64, input message_models.CreateMessageInput, isInternal bool) (message_models.CreateMessageOutput, error) {
	// 1. SÉCURITÉ DES TYPES DE MESSAGES (Filtre anti-usurpation)
	if !isInternal {
		switch input.MessageType {
		case 0, 2, 3: // Texte, Image, GIF : OK
		case 1, 4: // Vocales, Vidéos : En attente d'implémentation
			return message_models.CreateMessageOutput{}, nubo_error.NewBadRequest("UNSUPPORTED_MESSAGE_TYPE", "Les messages vocaux et vidéos ne sont pas encore supportés.", nil)
		case 5, 6, 7, 8: // Systèmes, Invitations, Liens
			return message_models.CreateMessageOutput{}, nubo_error.NewForbidden("INVALID_MESSAGE_TYPE", "Vous n'avez pas l'autorisation d'envoyer ce type de message.", nil)
		default:
			return message_models.CreateMessageOutput{}, nubo_error.NewBadRequest("UNKNOWN_MESSAGE_TYPE", "Type de message inconnu.", nil)
		}
	}

	// 2. SÉCURITÉ : L'utilisateur doit être membre actif
	mem, err := security_service.LeftMember(ctx, convID, senderID)
	if err != nil {
		return message_models.CreateMessageOutput{}, err
	}
	if mem.Role < 0 {
		return message_models.CreateMessageOutput{}, nubo_error.NewForbidden("USER_BANNED", "Accès refusé : vous êtes banni de cette conversation.", nil)
	}

	// =========================================================================
	// LE VIDEUR INTRAITABLE (Rejet HTTP si mute actif)
	// =========================================================================
	nowMs := service.NowMillis()
	if mem.Settings.RestrictedUntil > nowMs {
		return message_models.CreateMessageOutput{}, nubo_error.NewForbidden(
			"MEMBER_MUTED",
			"Vous êtes actuellement muet dans cette conversation.",
			nil,
		)
	}

	// =========================================================================
	// LAZY EVALUATION (Auto-guérison si mute expiré mais toujours en BDD)
	// =========================================================================
	if mem.Settings.RestrictedUntil > 0 && mem.Settings.RestrictedUntil <= nowMs {
		// Le temps a fait son travail. On nettoie.
		mem.Settings.RestrictedUntil = 0
		mem.UpdatedAt = nowMs

		// A. Mise à jour L1 instantanée pour la suite de l'exécution
		memberID := fmt.Sprintf("%d:%d", convID, senderID)
		var memLite lite_models.MemberLiteRequest
		if errMem := redis.ConvMembers.GetObject(ctx, memberID, &memLite); errMem == nil {
			memLite.Settings.RestrictedUntil = 0
			_ = redis.ConvMembers.SetObject(ctx, memberID, memLite)
		}

		// B. Envoi asynchrone (Write-Behind) pour nettoyer Postgres/Mongo silencieusement
		_ = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)

		// C. Broadcast optionnel (Pour faire sauter l'icône "Muet" sur l'UI des autres)
		_ = realtime_service.BroadcastToConversation(context.Background(), convID, "member.unmuted", mem)
	}
	// === FIN DU LAZY EVALUATION ===

	// === NOUVEAU : CHARGEMENT DE LA CONVERSATION ET MATRICE DE PERMISSIONS (Cascade L1 -> L2 -> L3) ===
	conv, errConv := object_cache_service.GetConversationFromObjectCache(ctx, convID)
	if errConv != nil || conv.ID == 0 {
		conv, errConv = mongo.MongoGetConversation(convID)
		if errConv != nil || conv.ID == 0 {
			// FALLBACK ABSOLU L3
			conv, errConv = postgres.FuncGetConversation(ctx, convID)
			if errConv != nil || conv.ID == 0 {
				return message_models.CreateMessageOutput{}, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation introuvable.", errConv)
			}

			// ⬆️ PROMOTION L3 -> L2 (Asynchrone via la queue pour protéger Mongo)
			go func(c conversation_models.ConversationPayload) {
				bgCtx := context.Background()
				_ = redis.EnqueueDB(bgCtx, c.ID, c.ID, redis.EntityConversation, redis.ActionUpdate, c, redis.TargetMongo)
			}(conv)
		}

		// ⬆️ PROMOTION L3/L2 -> L1 (Immédiat en RAM)
		_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
	}

	if !isInternal && conv.Type > 0 {
		// Droit d'écriture (1 = Admins seuls, 2 = Annonces & Threads)
		if conv.Settings.WritePermission == 1 && mem.Role == 0 {
			return message_models.CreateMessageOutput{}, nubo_error.NewForbidden("WRITE_PERMISSION_DENIED", "Seuls les administrateurs peuvent envoyer des messages ici.", nil)
		}
		if conv.Settings.WritePermission == 2 && mem.Role == 0 {
			// On autorise si ThreadParentID > 0 (c'est une réponse à un thread)
			if input.ThreadParentID == 0 {
				return message_models.CreateMessageOutput{}, nubo_error.NewForbidden("WRITE_PERMISSION_DENIED", "Vous ne pouvez que répondre aux annonces dans cette communauté.", nil)
			}
		}

		// Droit d'envoi de médias
		if input.MessageType == 2 && conv.Settings.SendMediaPermission == 1 && mem.Role == 0 {
			return message_models.CreateMessageOutput{}, nubo_error.NewForbidden("MEDIA_PERMISSION_DENIED", "Vous n'êtes pas autorisé à envoyer des médias dans ce groupe.", nil)
		}
	}

	// 3. ACTIVATION DU MÉDIA (OUT-OF-BAND)
	attachMap := input.Attachments
	if attachMap == nil {
		attachMap = make(map[string]any)
	}

	if input.MessageType == 2 {
		// Logique d'activation de média inchangée...
		rawMediaID, exists := attachMap["media_id"]
		if !exists {
			return message_models.CreateMessageOutput{}, nubo_error.NewBadRequest("MISSING_MEDIA_ID", "L'identifiant du média (media_id) est manquant.", nil)
		}
		var mediaID int64
		switch v := rawMediaID.(type) {
		case float64:
			mediaID = int64(v)
		case int64:
			mediaID = v
		}

		if mediaID > 0 {
			if errAct := media_service.ActivateMediaBatch(ctx, []int64{mediaID}, senderID); errAct != nil {
				return message_models.CreateMessageOutput{}, nubo_error.NewBadRequest("MEDIA_ACTIVATION_FAILED", "Impossible d'utiliser cette image.", errAct)
			}
		} else {
			return message_models.CreateMessageOutput{}, nubo_error.NewBadRequest("INVALID_MEDIA_ID", "L'identifiant du média est invalide.", nil)
		}
	} else {
		// Nettoyage avant vérification
		input.Content = pkg.CleanStr(input.Content)

		// Prévention stricte des "Messages Fantômes" (Texte vide)
		if input.Content == "" && input.MessageType == 0 {
			return message_models.CreateMessageOutput{}, nubo_error.NewBadRequest("EMPTY_MESSAGE", "Le message ne peut pas être vide.", nil)
		}
	}

	// === NOUVEAU : EXTRACTION DES MENTIONS ===
	// On le fait ici car on est certain que le texte est propre et validé.
	mentionedUserIDs := ExtractMentions(input.Content)

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
		CreatedAt:      domain.TimeToMillis(now),
		UpdatedAt:      domain.TimeToMillis(now),
	}

	// 5. MISE EN CACHE L1 IMMÉDIATE (Object Cache LFU)
	_ = object_cache_service.SetMessageInObjectCache(ctx, msgPayload)
	destinataires, _ := cache_service.ProcessNewMessageInSpeedCache(ctx, msgID, convID, senderID)
	conv, _ = object_cache_service.GetConversationFromObjectCache(ctx, convID)

	// 6. DISTRIBUTION TEMPS RÉEL (WebSockets)
	msgView := message_models.MessageView{
		MessagePayload: msgPayload,
	}

	// HYDRATATION CONDITIONNELLE DU DTO WEBSOCKET
	if userLite, errLite := cache_service.GetUserLite(ctx, senderID); errLite == nil {
		msgView.SenderUsername = userLite.Username
		if conv.Type == 2 || conv.Type == 3 {
			msgView.SenderAvatarCommunityID = userLite.ProfilePictureID
		} else {
			if userLite.ProfilePictureID > 0 {
				if avatarView, errAvatar := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, senderID, 0, senderID); errAvatar == nil {
					msgView.SenderAvatar = avatarView
				}
			}
		}
	}

	if conv.Type == 2 || conv.Type == 3 {
		_ = realtime_service.DistributeToCommunity(ctx, "message.created", msgView, convID)
	} else {
		_ = realtime_service.DistributeToUsers(ctx, "message.created", msgView, destinataires)
	}

	// === 7. ROUTAGE DES PUSH NOTIFICATIONS (La Matrice) ===
	dispatchPushNotifications(msgView, destinataires, mentionedUserIDs)

	// 8. BATCHING DES COMPTEURS BDD
	for _, uID := range destinataires {
		worker.RegisterUnread(convID, uID)
	}

	// 9. ENVOI À LA FILE ASYNCHRONE (Write-Behind)
	err = redis.EnqueueDB(ctx, msgID, convID, redis.EntityMessage, redis.ActionCreate, msgPayload, redis.TargetAll)
	if err != nil {
		return message_models.CreateMessageOutput{}, err
	}

	output := message_models.CreateMessageOutput{
		MessageID: msgID,
	}

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé TOUT À LA FIN de la fonction. Cela écrase tout timestamp qui aurait
	// pu être généré précédemment (par ex. à l'intérieur de AddMembersToConversation)
	// et garantit que le client reçoit la date de la fin absolue de la transaction.
	timestampMs := cache_service.TouchInboxActivity(ctx, senderID)
	output.InboxUpdateAt = domain.TimeToMillis(time.UnixMilli(timestampMs))

	return output, nil
}
