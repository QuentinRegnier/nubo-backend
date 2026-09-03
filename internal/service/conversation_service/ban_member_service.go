package conversation_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// BanMember expulse et bannit un membre d'une conversation (Rôle = -2).
func BanMember(ctx context.Context, callerID int64, input conversation_models.BanMemberInput) error {
	// 1. SÉCURITÉ : Vérification des droits du Caller (L1 -> L2 -> L3)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return nubo_error.NewForbidden("ACCESS_DENIED", "Conversation introuvable ou accès refusé.", err)
	}
	if callerMem.Role < 1 { // Doit être au moins Admin (1)
		return nubo_error.NewForbidden("INSUFFICIENT_PERMISSIONS", "Vous devez être administrateur ou propriétaire pour bannir un membre.", nil)
	}

	// 2. RÉCUPÉRATION DU MEMBRE CIBLE
	targetMem, err := security_service.LeftMember(ctx, input.ConversationID, input.TargetUserID)
	if err != nil {
		// S'il est déjà banni (-2) ou s'il a déjà quitté (-1), LeftMember renvoie une erreur.
		return nubo_error.NewBadRequest("USER_NOT_MEMBER", "L'utilisateur ciblé n'est pas un membre actif de ce groupe.", err)
	}

	// 3. VÉRIFICATION HIERARCHIQUE
	if callerMem.Role <= targetMem.Role {
		return nubo_error.NewForbidden("HIERARCHY_VIOLATION", "Vous ne pouvez pas bannir un membre de rang égal ou supérieur.", nil)
	}

	// 4. CRÉATION DU MESSAGE SYSTÈME (Type 8) AVANT LE GEL !
	callerLite, _ := cache_service.GetUserLite(ctx, callerID)
	targetLite, _ := cache_service.GetUserLite(ctx, input.TargetUserID)

	sysContent := fmt.Sprintf("%s has banned %s", callerLite.Username, targetLite.Username)
	msgInput := message_models.CreateMessageInput{
		MessageType: 8,
		Content:     sysContent,
	}

	// Expédition via le service Message.
	// Cela insère le message dans la BDD et met à jour les ZSETs des autres utilisateurs.
	msgID, errMsg := message_service.CreateMessage(ctx, callerID, input.ConversationID, msgInput, true)

	frozenID := msgID
	if errMsg != nil || msgID == 0 {
		// Fallback de sécurité (très rare) si la création du message échoue
		conv, _ := object_cache_service.GetConversationFromObjectCache(ctx, input.ConversationID)
		frozenID = conv.LastMessageID
	}

	// 5. APPLICATION DE LA MODIFICATION ET GEL DU TEMPS
	targetMem.Role = -2
	targetMem.FrozenMessageID = frozenID // L'historique s'arrêtera pile sur CE message !
	targetMem.UpdatedAt = time.Now().UTC()

	// 6. MISE À JOUR DE LA RAM ET ENVOI AUX WORKERS
	_ = object_cache_service.SetMemberInObjectCache(ctx, targetMem)

	// === NOUVEAU : MISE À JOUR SYNCHRONE DU SPEED CACHE ===
	// Retire le membre du Fan-Out et met à jour son rôle en RAM
	_ = cache_service.UpdateMemberSpeedCache(ctx, lite_models.MemberLiteRequest{
		ConversationID:  targetMem.ConversationID,
		UserID:          targetMem.UserID,
		Role:            targetMem.Role,
		UnreadCount:     targetMem.UnreadCount,
		FrozenMessageID: targetMem.FrozenMessageID,
		JoinedAt:        targetMem.JoinedAt.UnixMilli(),
	})

	err = redis.EnqueueDB(ctx, targetMem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, targetMem, redis.TargetAll)

	// Envoi notification (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), input.ConversationID, "member.banned", targetMem)
			if err != nil {
				_ = fmt.Errorf("Failed to broadcast member ban: %v", err)
			}
		}()
	}

	return err
}
