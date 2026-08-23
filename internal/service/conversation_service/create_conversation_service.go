package conversation_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// CreateConversation valide les règles, construit les entités et appelle l'ajout des membres en interne.
func CreateConversation(ctx context.Context, callerID int64, input conversation_models.CreateConversationInput) (conversation_models.CreateConversationOutput, error) {
	// 1. VALIDATION MÉTIER
	if input.Type == 2 || input.Type == 3 {
		return conversation_models.CreateConversationOutput{}, errors.New("la création manuelle de communautés est réservée au système ou aux administrateurs")
	}
	if input.Type == 0 && len(input.ParticipantIDs) != 1 {
		return conversation_models.CreateConversationOutput{}, errors.New("un message privé doit comporter exactement un participant cible")
	}
	if input.Type == 1 && input.Title == "" {
		return conversation_models.CreateConversationOutput{}, errors.New("les groupes nécessitent un titre")
	}

	// 2. VÉRIFICATION DE LA CONFIDENTIALITÉ POUR LES MP (Type 0)
	if input.Type == 0 {
		targetID := input.ParticipantIDs[0]
		if targetID == callerID {
			return conversation_models.CreateConversationOutput{}, errors.New("vous ne pouvez pas créer de conversation avec vous-même")
		}

		targetLite, err := cache_service.GetUserLite(ctx, targetID)
		if err != nil || targetLite.ID == 0 {
			return conversation_models.CreateConversationOutput{}, errors.New("utilisateur cible introuvable")
		}

		relationState := cache_service.RelationValue(ctx, callerID, targetID)
		if relationState == -1 {
			return conversation_models.CreateConversationOutput{}, errors.New("action impossible : utilisateur bloqué")
		}

		canCommunicate := false
		switch targetLite.ConversationPermission {
		case 0:
			canCommunicate = true
		case 1:
			canCommunicate = relationState >= 1
		case 2:
			canCommunicate = relationState == 2
		}

		if !canCommunicate {
			return conversation_models.CreateConversationOutput{}, errors.New("cet utilisateur n'accepte pas les messages directs")
		}
	}

	// 3. PRÉPARATION DES DONNÉES DE LA CONVERSATION
	now := time.Now().UTC()
	convID := pkg.GenerateID()

	title := ""
	if input.Type > 0 {
		title = pkg.CleanStr(input.Title)
	}

	convPayload := conversation_models.ConversationPayload{
		ID:        convID,
		Type:      input.Type,
		Title:     title,
		State:     0,
		Laws:      []int{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	_ = object_cache_service.SetConversationInObjectCache(ctx, convPayload)

	err := redis.EnqueueDB(ctx, convID, convID, redis.EntityConversation, redis.ActionCreate, convPayload, redis.TargetAll)
	if err != nil {
		return conversation_models.CreateConversationOutput{}, err
	}

	output := conversation_models.CreateConversationOutput{
		ConversationID: convID,
	}

	// 4. CRÉATION DES MEMBRES INITIAUX ET DÉLÉGATION
	if input.Type == 0 {
		// --- CRÉATION MP (Type 0) ---
		membersToAdd := []int64{callerID, input.ParticipantIDs[0]}
		for _, userID := range membersToAdd {
			mem := conversation_models.MemberPayload{
				ID:             pkg.GenerateID(),
				ConversationID: convID,
				UserID:         userID,
				Role:           0,
				JoinedAt:       now,
				UnreadCount:    0,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			_ = object_cache_service.SetMemberInObjectCache(ctx, mem)

			// === NOUVEAU : MISE À JOUR SYNCHRONE DU SPEED CACHE ===
			_ = cache_service.AddMemberToSpeedCache(ctx, models.MemberLiteRequest{
				ConversationID: mem.ConversationID,
				UserID:         mem.UserID,
				Role:           mem.Role,
				UnreadCount:    mem.UnreadCount,
			})

			// ENVOI NOTIFICATION
			_ = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionCreate, mem, redis.TargetAll)
		}

		_ = realtime_service.DistributeToUsers(ctx, "conversation.created", convPayload, membersToAdd)
	} else if input.Type == 1 {
		// --- CRÉATION GROUPE (Type 1) ---
		mem := conversation_models.MemberPayload{
			ID:              pkg.GenerateID(),
			ConversationID:  convID,
			UserID:          callerID,
			Role:            2, // Propriétaire
			JoinedAt:        now,
			FrozenMessageID: 0,
			UnreadCount:     0,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_ = object_cache_service.SetMemberInObjectCache(ctx, mem)

		// === NOUVEAU : MISE À JOUR SYNCHRONE DU SPEED CACHE ===
		_ = cache_service.AddMemberToSpeedCache(ctx, models.MemberLiteRequest{
			ConversationID: mem.ConversationID,
			UserID:         mem.UserID,
			Role:           mem.Role,
			UnreadCount:    mem.UnreadCount,
			JoinedAt:       mem.JoinedAt.UnixMilli(),
		})

		_ = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionCreate, mem, redis.TargetAll)

		// Appel direct de la fonction qui est désormais dans le même package !
		if len(input.ParticipantIDs) > 0 {
			addInput := conversation_models.AddMemberInput{
				ConversationID: convID,
				ParticipantIDs: input.ParticipantIDs,
			}
			addOutput, _ := AddMembersToConversation(ctx, callerID, addInput)

			output.AddedUserIDs = addOutput.AddedUserIDs
			output.InvitedUserIDs = addOutput.InvitedUserIDs
			output.RejectedUserIDs = addOutput.RejectedUserIDs
			output.MessageIDs = addOutput.MessageIDs           // Propagation des IDs générés
			output.ConversationIDs = addOutput.ConversationIDs // Propagation des canaux utilisés
		}

		// ENVOI NOTIFICATION
		_ = realtime_service.DistributeToUsers(ctx, "conversation.created", convPayload, []int64{callerID})
	}

	return output, nil
}
