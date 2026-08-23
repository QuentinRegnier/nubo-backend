package conversation_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

func JoinGroup(ctx context.Context, callerID int64, input conversation_models.JoinGroupInput) error {
	// 1. AUTO-GUÉRISON (Cascade L1 -> L2 -> L3) POUR VÉRIFIER LA CONVERSATION
	conv, err := object_cache_service.GetConversationFromObjectCache(ctx, input.ConversationID)
	if err != nil || conv.ID == 0 {
		conv, err = mongo.MongoGetConversation(input.ConversationID)
		if err != nil || conv.ID == 0 {
			conv, err = postgres.FuncGetConversation(ctx, input.ConversationID)
			if err != nil || conv.ID == 0 {
				return errors.New("la conversation n'existe pas ou a été supprimée")
			}
			_ = mongo.MongoUpsertConversation(conv)
		}
		_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
	}

	// 2. RÈGLE MÉTIER : On ne rejoint pas un MP.
	if conv.Type == 0 {
		return errors.New("impossible de rejoindre un message privé")
	}

	// ========================================================================
	// 3. LA BARRIÈRE DE SÉCURITÉ (Protection IDOR & Access Control)
	// ========================================================================
	// Type 1 = Groupe, Type 2 = Communauté Privée, Type 3 = Communauté Publique
	if conv.Type == 1 || conv.Type == 2 {
		if input.InviteMsgID == 0 {
			return errors.New("une invitation est requise pour rejoindre ce groupe privé")
		}

		// A. On s'assure que le caller a bien LE DROIT de lire ce message (c'est le sien)
		inviteMsg, errSec := security_service.LeftMessage(ctx, input.InviteMsgID, callerID)
		if errSec != nil {
			return errors.New("invitation introuvable ou vous n'en êtes pas le destinataire")
		}

		// B. On vérifie que c'est bien une invitation (Type 6)
		if inviteMsg.MessageType != 6 {
			return errors.New("le message fourni n'est pas une invitation valide")
		}

		// C. On vérifie que l'invitation correspond bien au groupe ciblé (Prévention d'usurpation)
		if inviteMsg.Attachments == nil {
			return errors.New("invitation corrompue (aucune cible)")
		}

		// Extraction robuste du JSONB qui désérialise souvent les nombres en float64
		targetConvRaw, exists := inviteMsg.Attachments["conversation_id"]
		if !exists {
			return errors.New("invitation invalide (cible manquante)")
		}

		var targetConvID int64
		switch v := targetConvRaw.(type) {
		case float64:
			targetConvID = int64(v)
		case int64:
			targetConvID = v
		}

		if targetConvID != input.ConversationID {
			return errors.New("cette invitation ne correspond pas à ce groupe")
		}
	}
	// Si Type == 3, la porte est ouverte, on passe directement à la suite.

	// ========================================================================
	// 4. VÉRIFICATION DU STATUT DU MEMBRE
	// ========================================================================
	var mem conversation_models.MemberPayload
	var isUpdate bool

	mem, err = object_cache_service.GetMemberFromObjectCache(ctx, input.ConversationID, callerID)
	if err != nil || mem.ID == 0 {
		memPg, errPg := postgres.FuncGetMember(ctx, input.ConversationID, callerID)
		if errPg == nil && memPg.ID != 0 {
			mem = memPg
		}
	}

	if mem.ID != 0 {
		if mem.Role == -2 {
			return errors.New("vous êtes banni de ce groupe")
		}
		if mem.Role >= 0 {
			return errors.New("vous faites déjà partie de ce groupe")
		}
		// Role = -1 : L'utilisateur revient après avoir quitté.
		isUpdate = true
	}

	// 5. CONSTRUCTION DE L'ÉTAT
	now := time.Now().UTC()
	if !isUpdate {
		mem = conversation_models.MemberPayload{
			ID:              pkg.GenerateID(),
			ConversationID:  input.ConversationID,
			UserID:          callerID,
			Role:            0, // Membre standard
			JoinedAt:        now,
			UnreadCount:     0,
			FrozenMessageID: 0,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	} else {
		mem.Role = 0
		mem.JoinedAt = now
		mem.UpdatedAt = now
		mem.FrozenMessageID = 0
	}

	// 6. CACHE L1 IMMÉDIAT
	_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
	_ = cache_service.AddConversationToUserInbox(ctx, callerID, conv.ID, conv.LastMessageID)

	// === NOUVEAU : MISE À JOUR SYNCHRONE DU SPEED CACHE ===
	memLite := models.MemberLiteRequest{
		ConversationID:  mem.ConversationID,
		UserID:          mem.UserID,
		Role:            mem.Role,
		UnreadCount:     mem.UnreadCount,
		FrozenMessageID: mem.FrozenMessageID,
		JoinedAt:        mem.JoinedAt.UnixMilli(),
	}
	if isUpdate {
		_ = cache_service.UpdateMemberSpeedCache(ctx, memLite)
	} else {
		_ = cache_service.AddMemberToSpeedCache(ctx, memLite)
	}

	// 9. WRITE-BEHIND
	action := redis.ActionCreate
	if isUpdate {
		action = redis.ActionUpdate
	}
	err = redis.EnqueueDB(ctx, mem.ID, input.ConversationID, redis.EntityMembers, action, mem, redis.TargetAll)

	if err == nil {
		// 7. & 8. MESSAGE SYSTÈME ET NOTIFICATION (Asynchrone)
		go func() {
			bgCtx := context.Background()
			if callerLite, errLite := cache_service.GetUserLite(bgCtx, callerID); errLite == nil {
				sysContent := fmt.Sprintf("%s a rejoint le groupe", callerLite.Username)
				msgInput := message_models.CreateMessageInput{
					MessageType: 8,
					Content:     sysContent,
				}
				_, _ = message_service.CreateMessage(bgCtx, callerID, input.ConversationID, msgInput, true)
			}
			_ = realtime_service.BroadcastToConversation(bgCtx, input.ConversationID, "member.joined", mem)
		}()
	}

	return err
}
