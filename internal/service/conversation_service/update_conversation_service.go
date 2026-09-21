package conversation_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UpdateConversation gère la modification (Titre, Règles, Description, Avatar) d'un groupe ou d'une communauté
func UpdateConversation(ctx context.Context, callerID int64, convID int64, input conversation_models.UpdateConversationInput) (conversation_models.UpdateConversationOutput, error) {
	// 1. VÉRIFICATION SÉCURITÉ ET RÉCUPÉRATION (Objet Complet)
	conv, err := security_service.LeftConversation(ctx, convID, callerID)
	if err != nil {
		return conversation_models.UpdateConversationOutput{}, err // L'erreur est déjà formatée par LeftConversation
	}

	// 2. RÈGLES MÉTIER & BARRIÈRE DE SÉCURITÉ STRICTE
	if conv.Type == 0 {
		return conversation_models.UpdateConversationOutput{}, nubo_error.NewForbidden("INVALID_CONV_TYPE", "Impossible de modifier les métadonnées ou les paramètres d'un message privé.", nil)
	}

	// 3. APPLICATION DES MODIFICATIONS
	isTitleUpdated := false
	if input.Title != conv.Title {
		conv.Title = input.Title
		isTitleUpdated = true
	}

	isDescUpdated := false
	isAvatarUpdated := false
	isSettingsUpdated := false
	isLinkUpdated := false

	// VÉRIFICATION DES PARAMÈTRES (Settings)
	wasApprovalRequired := conv.Settings.JoinApprovalRequired

	// Si le Front envoie une modification des règles, on l'applique
	if input.Settings != conv.Settings {
		conv.Settings = input.Settings
		isSettingsUpdated = true
	}

	// Détecte le moment exact où on désactive l'approbation pour une communauté
	isApprovalDropped := conv.Type == 3 && wasApprovalRequired && !conv.Settings.JoinApprovalRequired

	// Vérification de l'exclusivité des fonctionnalités de Communauté (Type 3)
	if conv.Type == 3 {
		if input.Description != conv.Description {
			conv.Description = input.Description
			isDescUpdated = true
		}
		if input.AvatarID != conv.AvatarID {
			conv.AvatarID = input.AvatarID
			isAvatarUpdated = true
		}
		// GESTION DU LIEN EXTERNE
		if input.ExternalLink != conv.ExternalLink {
			conv.ExternalLink = input.ExternalLink
			isLinkUpdated = true
		}
	} else {
		// Bouclier : Rejet 403 si tentative d'injection sur un groupe classique
		if (input.Description != "" && input.Description != conv.Description) ||
			(input.AvatarID != 0 && input.AvatarID != conv.AvatarID) ||
			(input.ExternalLink.URL != "" || input.ExternalLink.Title != "") {
			return conversation_models.UpdateConversationOutput{}, nubo_error.NewForbidden("FEATURE_NOT_SUPPORTED", "Seules les communautés publiques peuvent posséder une description, un avatar ou un lien externe.", nil)
		}
	}

	conv.UpdatedAt = domain.NowMillis()

	// === MISE À JOUR IMMÉDIATE DU L1 (Object Cache) ===
	_ = object_cache_service.SetConversationInObjectCache(ctx, conv)

	// 4. DÉLÉGATION À LA FILE ASYNCHRONE (Write-Behind)
	// PartitionKey = convID pour garantir que cet Update passe après la Création dans la BDD
	err = redis.EnqueueDB(ctx, conv.ID, conv.ID, redis.EntityConversation, redis.ActionUpdate, conv, redis.TargetAll)
	if err != nil {
		return conversation_models.UpdateConversationOutput{}, err
	}

	// 5. MISE À JOUR IMMÉDIATE DU L1 (Uniquement si une metadata visuelle ou de règles a changé pour l'UI)
	if isTitleUpdated || isDescUpdated || isAvatarUpdated || isSettingsUpdated || isLinkUpdated {
		convLite := lite_models.ConvLiteRequest{
			ID:            conv.ID,
			Type:          conv.Type,
			Title:         conv.Title,
			Description:   conv.Description,
			AvatarID:      conv.AvatarID,
			LastMessageID: conv.LastMessageID,
			Settings:      service.ToConversationSettingsLite(conv.Settings),
			ExternalLink: models.ExternalLinks{
				Title: conv.ExternalLink.Title,
				URL:   conv.ExternalLink.URL,
			},
		}
		_ = redis.ConvMeta.SetObject(ctx, conv.ID, convLite)
	}

	// 6. ENVOI NOTIFICATION ET GESTION DES MEMBRES EN ATTENTE (Asynchrone)
	go func() {
		bgCtx := context.Background() // Le context background protège la routine de la fin de la requête HTTP

		// 1. Notification du changement de règles à tout le groupe
		errWS := realtime_service.BroadcastToConversation(bgCtx, conv.ID, "conversation.updated", conv)
		if errWS != nil {
			logger.Log.Error().Err(errWS).Msg("Erreur lors de l'envoi de la notification de mise à jour de conversation")
		}

		// 2. Traitement en masse des candidatures si la règle a été levée
		if isApprovalDropped {
			acceptAllPendingMembers(bgCtx, conv.ID, callerID)
		}
	}()

	output := conversation_models.UpdateConversationOutput{}

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	output.InboxUpdateAt = domain.TimeToMillis(time.UnixMilli(timestampMs))

	return output, nil
}

// acceptAllPendingMembers est une routine de fond qui pagine et accepte les membres en attente (Rôle = -3).
func acceptAllPendingMembers(ctx context.Context, convID int64, callerID int64) {
	limit := 50

	for {
		var members []member_models.MemberPayload

		// A. On tente d'abord le L2
		mongoMembers, errMongo := mongo.MongoLoadMembersByRolePaginated(convID, -3, int64(limit), 0)
		if errMongo == nil && len(mongoMembers) > 0 {
			members = mongoMembers
		} else {
			// B. Fallback L3
			pgMembers, errPg := postgres.FuncLoadMembersByRolePaginated(ctx, convID, -3, int64(limit), 0)
			if errPg == nil && len(pgMembers) > 0 {
				members = pgMembers
			}
		}

		// Condition d'arrêt de la boucle infinie : plus aucun membre à traiter
		if len(members) == 0 {
			break
		}

		// Note sur l'offset :
		// Puisqu'on modifie le rôle (de -3 à 0), les membres disparaissent du résultat de la requête.
		// On laisse donc toujours l'offset à 0 (on "consomme" le haut de la pile à chaque itération).

		for _, targetMem := range members {
			now := domain.NowMillis()
			targetMem.Role = 0 // Devient membre officiel
			targetMem.Settings = member_models.DefaultMemberSettings(3)
			targetMem.JoinedAt = now
			targetMem.UpdatedAt = now

			// 1. MISE À JOUR SYNCHRONE DES CACHES (L1)
			_ = object_cache_service.SetMemberInObjectCache(ctx, targetMem)
			_ = cache_service.UpdateMemberSpeedCache(ctx, lite_models.MemberLiteRequest{
				ConversationID:  targetMem.ConversationID,
				UserID:          targetMem.UserID,
				Role:            targetMem.Role,
				Settings:        service.ToMemberSettingsLite(targetMem.Settings),
				UnreadCount:     targetMem.UnreadCount,
				FrozenMessageID: targetMem.FrozenMessageID,
				JoinedAt:        targetMem.JoinedAt,
			})

			// 2. ENVOI AUX WORKERS (Write-Behind)
			_ = redis.EnqueueDB(ctx, targetMem.ID, convID, redis.EntityMembers, redis.ActionUpdate, targetMem, redis.TargetAll)

			// 3. MESSAGE SYSTÈME ET NOTIFICATION (Mode Twitch - Zéro crypto)
			if targetLite, errLite := cache_service.GetUserLite(ctx, targetMem.UserID); errLite == nil {
				sysContent := fmt.Sprintf("%s a rejoint le groupe", targetLite.Username)
				msgInput := message_models.CreateMessageInput{
					MessageType: 8,
					Content:     sysContent,
				}
				_, _ = message_service.CreateMessage(ctx, callerID, convID, msgInput, true)

				memView := member_models.MemberView{
					MemberPayload:     targetMem,
					Username:          targetLite.Username,
					IsOnline:          cache_service.IsUserOnline(ctx, targetMem.UserID),
					AvatarCommunityID: targetLite.ProfilePictureID, // Mode Twitch pour les communautés
				}

				_ = realtime_service.BroadcastToConversation(ctx, convID, "member.joined", memView)
			}
		}
	}
}
