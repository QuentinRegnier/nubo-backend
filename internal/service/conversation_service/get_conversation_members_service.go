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
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// GetConversationMembers récupère la liste détaillée des membres pour un lot de conversations.
func GetConversationMembers(ctx context.Context, callerID int64, input conversation_models.GetConversationMembersInput) ([]conversation_models.ConversationMembersList, error) {
	var results []conversation_models.ConversationMembersList

	for _, convID := range input.ConversationIDs {
		// 1. SÉCURITÉ : Vérification d'appartenance
		// On ignore silencieusement les conversations interdites ou introuvables.
		if _, err := security_service.LeftMember(ctx, convID, callerID); err != nil {
			continue
		}

		// 2. RÉCUPÉRATION DU TYPE DE CONVERSATION (Pour savoir comment hydrater l'avatar)
		conv, errConv := object_cache_service.GetConversationFromObjectCache(ctx, convID)
		if errConv != nil || conv.ID == 0 {
			conv, _ = mongo.MongoGetConversation(convID)
			if conv.ID == 0 {
				conv, _ = postgres.FuncGetConversation(ctx, convID)
			}
		}

		// 3. RÉCUPÉRATION DES IDs DES PARTICIPANTS (L1 -> L3 avec Auto-guérison)
		var pIDs []int64
		participantsStr, errPart := redis.ConvParticipants.SMembers(ctx, convID)

		if errPart == nil && len(participantsStr) > 0 {
			for _, pStr := range participantsStr {
				if id, err := strconv.ParseInt(pStr, 10, 64); err == nil {
					pIDs = append(pIDs, id)
				}
			}
		} else {
			// Fallback L3 (Postgres)
			pIDs, _ = postgres.FuncGetConversationParticipantIDs(ctx, convID)
			// Guérison L1 (Redis Set)
			for _, id := range pIDs {
				_ = redis.ConvParticipants.SAdd(ctx, convID, id)
			}
		}

		// 4. HYDRATATION DE CHAQUE MEMBRE
		var membersView []conversation_models.MemberView
		for _, pID := range pIDs {
			// A. Récupération du MemberPayload (L1 -> L2 -> L3)
			mem, errMem := object_cache_service.GetMemberFromObjectCache(ctx, convID, pID)
			if errMem != nil || mem.ID == 0 {
				mem, errMem = mongo.MongoGetMember(convID, pID)
				if errMem != nil || mem.ID == 0 {
					mem, _ = postgres.FuncGetMember(ctx, convID, pID)
					if mem.ID != 0 {
						_ = mongo.MongoUpsertMember(mem)
					}
				}
				if mem.ID != 0 {
					_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
				}
			}

			// On ignore ceux qui ont quitté ou sont bannis
			if mem.ID == 0 || mem.Role < 0 {
				continue
			}

			// B. Préparation de la vue et statut en ligne
			view := conversation_models.MemberView{
				MemberPayload: mem,
				IsOnline:      cache_service.IsUserOnline(ctx, pID), // NOUVEAU (O(1))
			}

			// C. Hydratation (Pseudo + Avatar Conditionnel)
			if targetLite, errLite := cache_service.GetUserLite(ctx, pID); errLite == nil {
				view.Username = targetLite.Username

				if conv.Type == 2 || conv.Type == 3 {
					view.AvatarCommunityID = targetLite.ProfilePictureID // Mode Twitch
				} else {
					if targetLite.ProfilePictureID > 0 {
						// Mode Classique : URL HMAC
						if avatarView, errMedia := media_service.GenerateMediaViewCascade(ctx, targetLite.ProfilePictureID, pID, 0, callerID); errMedia == nil {
							view.Avatar = avatarView
						}
					}
				}
			}

			membersView = append(membersView, view)
		}

		results = append(results, conversation_models.ConversationMembersList{
			ConversationID: convID,
			Members:        membersView,
		})
	}

	// Évite le `null` en JSON si l'utilisateur demande des IDs erronés
	if results == nil {
		results = make([]conversation_models.ConversationMembersList, 0)
	}

	return results, nil
}
