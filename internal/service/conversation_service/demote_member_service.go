package conversation_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// DemoteMember rétrograde un administrateur au rang de membre standard (Rôle = 0)
func DemoteMember(ctx context.Context, callerID int64, input conversation_models.DemoteMemberInput) error {
	// 1. SÉCURITÉ : Vérification des droits du Caller (L1 -> L2 -> L3)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return errors.New("conversation introuvable ou accès refusé")
	}
	if callerMem.Role != 2 {
		return errors.New("action refusée : seul le propriétaire peut destituer un administrateur")
	}

	// 2. RÉCUPÉRATION DU MEMBRE CIBLE
	targetMem, err := security_service.LeftMember(ctx, input.ConversationID, input.TargetUserID)
	if err != nil || targetMem.Role < 0 {
		return errors.New("l'utilisateur ciblé n'est pas membre de ce groupe")
	}

	// 3. RÈGLES MÉTIER ET IDEMPOTENCE
	if targetMem.Role == 0 {
		return nil // Déjà membre normal, on valide silencieusement
	}
	if targetMem.Role == 2 {
		return errors.New("impossible de destituer le propriétaire, transférez d'abord la propriété")
	}

	// 4. APPLICATION DE LA MODIFICATION
	targetMem.Role = 0
	targetMem.UpdatedAt = time.Now().UTC()

	targetMem.Role = 1
	targetMem.UpdatedAt = time.Now().UTC()

	callerLite, _ := cache_service.GetUserLite(ctx, callerID)
	targetLite, _ := cache_service.GetUserLite(ctx, input.TargetUserID)

	sysContent := fmt.Sprintf("%s has promote %s", callerLite.Username, targetLite.Username)
	msgInput := message_models.CreateMessageInput{
		MessageType: 8,
		Content:     sysContent,
	}

	// Expédition via le service Message.
	// Cela insère le message dans la BDD et met à jour les ZSETs des autres utilisateurs.
	if _, errM := message_service.CreateMessage(ctx, callerID, input.ConversationID, msgInput, true); errM != nil {
		return errM
	}

	// 5. MISE À JOUR DE LA RAM (L1)
	_ = object_cache_service.SetMemberInObjectCache(ctx, targetMem)

	// === NOUVEAU : MISE À JOUR SYNCHRONE DU SPEED CACHE ===
	_ = cache_service.UpdateMemberSpeedCache(ctx, models.MemberLiteRequest{
		ConversationID:  targetMem.ConversationID,
		UserID:          targetMem.UserID,
		Role:            targetMem.Role,
		UnreadCount:     targetMem.UnreadCount,
		FrozenMessageID: targetMem.FrozenMessageID,
		JoinedAt:        targetMem.JoinedAt.UnixMilli(),
	})

	// 6. ENVOI AUX WORKERS (Write-Behind)
	err = redis.EnqueueDB(ctx, targetMem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, targetMem, redis.TargetAll)

	// 7. ENVOI NOTIFICATION (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), input.ConversationID, "member.demoted", targetMem)
			if err != nil {
				_ = fmt.Errorf("Failed to broadcast member demotion: %v", err)
			}
		}()
	}

	return err
}
