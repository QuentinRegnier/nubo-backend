package conversation_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// GetConversations récupère les métadonnées fraîches d'un lot de conversations spécifiques.
func GetConversations(ctx context.Context, callerID int64, input conversation_models.GetConversationsInput) (conversation_models.GetInboxOutput, error) {
	var views []conversation_models.InboxConversationView
	callerIDStr := strconv.FormatInt(callerID, 10)

	for _, convID := range input.ConversationIDs {
		// 1. SÉCURITÉ : Vérification d'appartenance
		// On ignore silencieusement les conversations interdites ou introuvables (pour ne pas bloquer le reste du tableau)
		mem, err := security_service.LeftMember(ctx, convID, callerID)
		if err != nil {
			continue
		}

		// 2. RÉCUPÉRATION DE LA CONVERSATION (Cascade L1 -> L2 -> L3 avec Auto-Guérison)
		conv, errConv := object_cache_service.GetConversationFromObjectCache(ctx, convID)
		if errConv != nil || conv.ID == 0 {
			conv, errConv = mongo.MongoGetConversation(convID)
			if errConv != nil || conv.ID == 0 {
				conv, errConv = postgres.FuncGetConversation(ctx, convID)
				if errConv != nil || conv.ID == 0 {
					continue
				}
				// ⬆️ PROMOTION L3 -> L2 (Asynchrone via la queue)
				go func(c conversation_models.ConversationPayload) {
					bgCtx := context.Background()
					_ = redis.EnqueueDB(bgCtx, c.ID, c.ID, redis.EntityConversation, redis.ActionUpdate, c, redis.TargetMongo)
				}(conv)
			}
			// ⬆️ PROMOTION L3/L2 -> L1 (Immédiat)
			_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
		}

		// 3. HYDRATATION DYNAMIQUE DU TITRE (Pour les MP - Type 0)
		title := conv.Title
		isOnline := false // Par défaut hors-ligne

		if conv.Type == 0 {
			participantsStr, errPart := redis.ConvParticipants.SMembers(ctx, conv.ID)
			if errPart == nil {
				for _, pStr := range participantsStr {
					if pStr != callerIDStr {
						// On isole l'autre participant
						if otherID, errParse := strconv.ParseInt(pStr, 10, 64); errParse == nil {
							// 1. Fetch de son pseudo
							if otherLite, errLite := cache_service.GetUserLite(ctx, otherID); errLite == nil {
								title = otherLite.Username
							}
							// 2. Fetch de sa présence (NOUVEAU)
							isOnline = cache_service.IsUserOnline(ctx, otherID)
							break
						}
					}
				}
			}
		}

		// 4. CALCUL DYNAMIQUE DES AVATARS
		avatars := GetConversationAvatars(ctx, conv.ID, callerID, conv.Type)

		// 5. ASSEMBLAGE DE LA VUE
		views = append(views, conversation_models.InboxConversationView{
			ConversationID: conv.ID,
			Type:           conv.Type,
			Title:          title,
			Description:    conv.Description, // NOUVEAU
			AvatarID:       conv.AvatarID,    // NOUVEAU
			LastMessageID:  conv.LastMessageID,
			Role:           mem.Role,
			Settings:       mem.Settings,
			UnreadCount:    mem.UnreadCount,
			Avatars:        avatars,
			IsOnline:       isOnline,
		})
	}

	// Prévention du `null` en JSON si le tableau est vide
	if views == nil {
		views = make([]conversation_models.InboxConversationView, 0)
	}

	return conversation_models.GetInboxOutput{Conversations: views}, nil
}
