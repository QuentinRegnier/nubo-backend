package conversation_service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
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

	// === NOUVEAU : VÉRIFICATION DES DROITS D'AJOUT ===
	if !conv.Settings.AddMemberPermission && callerMem.Role == 0 {
		return conversation_models.AddMemberOutput{}, nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Seuls les administrateurs peuvent ajouter des membres à ce groupe.", nil)
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
			memberPayload := member_models.MemberPayload{
				ID:                pkg.GenerateID(),
				ConversationID:    conv.ID,
				UserID:            targetID,
				Role:              0, // Membre standard
				Settings:          member_models.DefaultMemberSettings(conv.Type),
				JoinedAt:          domain.TimeToMillis(now),
				FrozenMessageID:   0,
				LastReadMessageID: 0,
				UnreadCount:       0,
				CreatedAt:         domain.TimeToMillis(now),
				UpdatedAt:         domain.TimeToMillis(now),
			}

			_ = object_cache_service.SetMemberInObjectCache(ctx, memberPayload)
			_ = cache_service.AddMemberToSpeedCache(ctx, lite_models.MemberLiteRequest{
				ConversationID:    conv.ID,
				UserID:            targetID,
				Role:              0,
				Settings:          service.ToMemberSettingsLite(member_models.DefaultMemberSettings(conv.Type)),
				FrozenMessageID:   0,
				LastReadMessageID: 0,
				UnreadCount:       0,
				JoinedAt:          memberPayload.JoinedAt,
			})

			_ = redis.EnqueueDB(ctx, memberPayload.ID, conv.ID, redis.EntityMembers, redis.ActionCreate, memberPayload, redis.TargetAll)

			// === NOUVEAU : FILTRAGE DES MESSAGES SYSTÈMES ===
			callerLite, _ := cache_service.GetUserLite(ctx, callerID)
			sysContent := fmt.Sprintf("%s a ajouté %s", callerLite.Username, targetLite.Username)
			msgInput := message_models.CreateMessageInput{
				MessageType: 8,
				Content:     sysContent,
			}
			// Expédition via le service Message.
			_, _ = message_service.CreateMessage(ctx, callerID, input.ConversationID, msgInput, true)

			output.AddedUserIDs = append(output.AddedUserIDs, targetID)

			// Envoie notification (Asynchrone)
			go func(payload member_models.MemberPayload, cID int64, tID int64, cType int, cCaller int64) {
				bgCtx := context.Background()

				isOnline := cache_service.IsUserOnline(bgCtx, payload.UserID)
				if targetLite.ShowOnlineStatus == false {
					isOnline = false
				}

				memView := member_models.MemberView{
					MemberPayload: payload,
					IsOnline:      isOnline,
				}

				if targetLite, errLite := cache_service.GetUserLite(bgCtx, payload.UserID); errLite == nil {
					memView.Username = targetLite.Username

					if cType == 2 || cType == 3 {
						memView.AvatarCommunityID = targetLite.ProfilePictureID
					} else {
						if targetLite.ProfilePictureID > 0 {
							if avatarView, errMedia := media_service.GenerateMediaViewCascade(bgCtx, targetLite.ProfilePictureID, payload.UserID, 0, cCaller); errMedia == nil {
								memView.Avatar = avatarView
							}
						}
					}
				}

				_ = realtime_service.BroadcastToConversation(bgCtx, cID, "member.added", memView)
				_ = realtime_service.DistributeToUsers(bgCtx, "conversation.created", conv, []int64{tID})

				// ✅ NOUVEAU : SYNC LEDGER (Trigger global)
				participantsStr, _ := redis.ConvParticipants.SMembers(bgCtx, cID)
				var pIDs []int64
				for _, p := range participantsStr {
					if id, err := strconv.ParseInt(p, 10, 64); err == nil {
						pIDs = append(pIDs, id)
					}
				}
				_ = cache_service.RecordConversationMutation(bgCtx, cID, pIDs)

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
				createMessageOutput, errCreate := message_service.CreateMessage(ctx, callerID, directConvID, msgInput, true)
				if errCreate == nil && createMessageOutput.MessageID > 0 {
					output.MessageIDs = append(output.MessageIDs, createMessageOutput.MessageID) // On trace chaque invitation envoyée
					output.ConversationIDs = append(output.ConversationIDs, directConvID)        // On trace le canal MP utilisé
					output.InvitedUserIDs = append(output.InvitedUserIDs, targetID)

					// === NOUVEAU : DÉCLENCHEUR GROUP_INVITED ===
					go func(tID int64, cID int64) {
						_ = notification_service.DispatchNotification(context.Background(), tID, callerID, "group_invited", cID)
					}(targetID, conv.ID)
				}
			}

			output.InvitedUserIDs = append(output.InvitedUserIDs, targetID)
		}
	}

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé TOUT À LA FIN de la fonction. Cela écrase tout timestamp qui aurait
	// pu être généré précédemment (par ex. à l'intérieur de AddMembersToConversation)
	// et garantit que le client reçoit la date de la fin absolue de la transaction.
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	output.InboxUpdateAt = domain.TimeToMillis(time.UnixMilli(timestampMs))

	return output, nil
}
