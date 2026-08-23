package conversation_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// MarkConversationAsRead remet le compteur de messages non lus à 0 pour l'utilisateur.
func MarkConversationAsRead(ctx context.Context, callerID int64, convID int64) error {
	// 1. SÉCURITÉ ET RÉCUPÉRATION (Objet Complet)
	mem, err := security_service.LeftMember(ctx, convID, callerID)
	if err != nil {
		return err
	}

	// 2. OPTIMISATION (Coupe-circuit)
	// Si le compteur est déjà à 0, on arrête tout : ça ne sert à rien de spammer les workers et PostgreSQL.
	if mem.UnreadCount == 0 {
		return nil
	}

	// 3. APPLICATION DES MODIFICATIONS
	mem.UnreadCount = 0
	mem.UpdatedAt = time.Now().UTC()

	// 4. DÉLÉGATION À LA FILE ASYNCHRONE (Write-Behind)
	// PartitionKey = convID pour garantir que l'ordre des requêtes est respecté pour cette conversation
	err = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)
	if err != nil {
		return err
	}

	// 5. MISE À JOUR IMMÉDIATE DU L1
	_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
	_ = cache_service.ResetMemberUnreadCountInSpeedCache(ctx, convID, callerID)

	// 6. ENVOI NOTIFICATION (Asynchrone)
	go func() {
		err := realtime_service.BroadcastToConversation(context.Background(), convID, "conversation.read_receipt", mem)
		if err != nil {
			_ = fmt.Errorf("MarkConversationAsRead: Erreur lors de l'envoi de la notification : %v", err)
		}
	}()

	return nil
}
