package conversation_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// AddMembersToConversation orchestre l'ajout de membres en appliquant la matrice de confidentialité L1.
func AddMembersToConversation(ctx context.Context, callerID int64, input conversation_models.AddMemberInput) (conversation_models.AddMemberOutput, error) {
	// 1. SÉCURITÉ : Le caller doit être membre actif du groupe (Role >= 0)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil || callerMem.Role < 0 {
		return conversation_models.AddMemberOutput{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Accès refusé : vous ne faites pas partie de cette conversation.", err)
	}

	// 2. RÉCUPÉRATION DU GROUPE
	conv, err := object_cache_service.GetConversationFromObjectCache(ctx, input.ConversationID)
	if err != nil || conv.State != 0 {
		return conversation_models.AddMemberOutput{}, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation introuvable ou inactive.", err)
	}

	// Interdiction d'ajouter des membres dans un Message Privé (Type 0)
	if conv.Type == 0 {
		return conversation_models.AddMemberOutput{}, nubo_error.NewForbidden("INVALID_CONV_TYPE", "Impossible d'ajouter des membres à une conversation privée à deux.", nil)
	}

	output := conversation_models.AddMemberOutput{
		AddedUserIDs:    make([]int64, 0),
		InvitedUserIDs:  make([]int64, 0),
		RejectedUserIDs: make([]int64, 0),
		MessageIDs:      make([]int64, 0),
		ConversationIDs: make([]int64, 0),
	}

	now := time.Now().UTC()

	// 3. BOUCLE DE VALIDATION SUR CHAQUE PARTICIPANT
	for _, targetID := range input.ParticipantIDs {
		if targetID == callerID {
			continue // Évite d'ajouter le caller une deuxième fois
		}

		// A. Lecture O(1) de l'empreinte de confidentialité dans le SPEED Cache L1
		targetLite, err := cache_service.GetUserLite(ctx, targetID)
		if err != nil || targetLite.ID == 0 {
			output.RejectedUserIDs = append(output.RejectedUserIDs, targetID)
			continue
		}

		// B. Calcul de la relation entre le Caller et la Cible (0=Rien, 1=Follow, 2=Ami, -1=Banni)
		// Ici la relation évaluée est : "Qu'est-ce que Target pense de Caller ?"
		relationState := cache_service.RelationValue(ctx, callerID, targetID)

		// RÈGLE DE BAN : Si l'un des deux a bloqué l'autre, rejet direct
		if relationState == -1 {
			output.RejectedUserIDs = append(output.RejectedUserIDs, targetID)
			continue
		}

		// VERROU 1 : Droit de communication (ConversationPermission)
		canCommunicate := false
		switch targetLite.ConversationPermission {
		case 0:
			canCommunicate = true
		case 1:
			canCommunicate = relationState >= 1 // L'utilisateur cible suit ou est ami avec l'appelant
		case 2:
			canCommunicate = relationState == 2 // L'utilisateur cible est ami avec l'appelant
		}

		if !canCommunicate {
			output.RejectedUserIDs = append(output.RejectedUserIDs, targetID)
			continue
		}

		// VERROU 2 : Mode d'ajout (AddGroupPermission)
		canAddDirectly := false
		switch targetLite.AddGroupPermission {
		case 0: // Tout le monde peut l'ajouter
			canAddDirectly = true
		case 1: // Seuls ses amis peuvent l'ajouter
			canAddDirectly = (relationState == 2)
		case 2: // Personne ne peut l'ajouter (invitation obligatoire)
			canAddDirectly = false
		}

		if canAddDirectly {
			// --- CAS A : AJOUT AUTOMATIQUE DIRECT ---
			memberPayload := conversation_models.MemberPayload{
				ID:              pkg.GenerateID(),
				ConversationID:  conv.ID,
				UserID:          targetID,
				Role:            0, // Membre standard
				Settings:        DefaultMemberSettings(conv.Type),
				JoinedAt:        now,
				FrozenMessageID: 0,
				UnreadCount:     0,
				CreatedAt:       now,
				UpdatedAt:       now,
			}

			_ = object_cache_service.SetMemberInObjectCache(ctx, memberPayload)
			_ = cache_service.AddMemberToSpeedCache(ctx, lite_models.MemberLiteRequest{
				ConversationID: conv.ID,
				UserID:         targetID,
				Role:           0,
				Settings:       service.ToMemberSettingsLite(DefaultMemberSettings(conv.Type)),
				UnreadCount:    0,
				JoinedAt:       memberPayload.JoinedAt.UnixMilli(),
			})

			_ = redis.EnqueueDB(ctx, memberPayload.ID, conv.ID, redis.EntityMembers, redis.ActionCreate, memberPayload, redis.TargetAll)

			callerLite, _ := cache_service.GetUserLite(ctx, callerID)

			sysContent := fmt.Sprintf("%s has add %s", callerLite.Username, targetLite.Username)
			msgInput := message_models.CreateMessageInput{
				MessageType: 8,
				Content:     sysContent,
			}

			// Expédition via le service Message.
			// Cela insère le message dans la BDD et met à jour les ZSETs des autres utilisateurs.
			if _, errM := message_service.CreateMessage(ctx, callerID, input.ConversationID, msgInput, true); errM != nil {
				return conversation_models.AddMemberOutput{}, errM
			}

			output.AddedUserIDs = append(output.AddedUserIDs, targetID)

			// Envoie notification (Asynchrone)
			go func(payload conversation_models.MemberPayload, cID int64, tID int64, cType int, cCaller int64) {
				bgCtx := context.Background()

				// HYDRATATION CONDITIONNELLE DU DTO WEBSOCKET
				memView := conversation_models.MemberView{
					MemberPayload: payload,
					IsOnline:      cache_service.IsUserOnline(bgCtx, payload.UserID), // NOUVEAU
				}

				if targetLite, errLite := cache_service.GetUserLite(bgCtx, payload.UserID); errLite == nil {
					memView.Username = targetLite.Username

					if cType == 2 || cType == 3 {
						memView.AvatarCommunityID = targetLite.ProfilePictureID // Twitch Mode
					} else {
						if targetLite.ProfilePictureID > 0 {
							// Mode Classique : URL HMAC signée
							if avatarView, errMedia := media_service.GenerateMediaViewCascade(bgCtx, targetLite.ProfilePictureID, payload.UserID, 0, cCaller); errMedia == nil {
								memView.Avatar = avatarView
							}
						}
					}
				}

				// Diffusion du MemberView au lieu du MemberPayload brut !
				_ = realtime_service.BroadcastToConversation(bgCtx, cID, "member.added", memView)

				// On notifie la cible qu'elle a une nouvelle conversation !
				_ = realtime_service.DistributeToUsers(bgCtx, "conversation.created", conv, []int64{tID})

			}(memberPayload, conv.ID, targetID, conv.Type, callerID)
		} else {
			// --- CAS B : ENVOI D'UNE INVITATION (Message Système Type 6) ---
			// 1. Récupération ou création de la conversation privée (MP) entre les deux utilisateurs
			directConvID, errMP := GetOrCreateDirectConversation(ctx, callerID, targetID)
			if errMP == nil {
				// 2. Construction du payload de l'invitation (Map native)
				attachmentsMap := map[string]any{
					"conversation_id": conv.ID,
				}

				msgInput := message_models.CreateMessageInput{
					MessageType: 6,  // Type 6 = Invitation
					Content:     "", // Vide, le front affichera une UI riche via les Attachments
					Attachments: attachmentsMap,
				}

				// 3. Expédition du message via le service Message (DDD total)
				msgID, errCreate := message_service.CreateMessage(ctx, callerID, directConvID, msgInput, true)
				if errCreate == nil && msgID > 0 {
					output.MessageIDs = append(output.MessageIDs, msgID)                  // On trace chaque invitation envoyée
					output.ConversationIDs = append(output.ConversationIDs, directConvID) // On trace le canal MP utilisé
				}
			}

			output.InvitedUserIDs = append(output.InvitedUserIDs, targetID)
		}
	}

	return output, nil
}
