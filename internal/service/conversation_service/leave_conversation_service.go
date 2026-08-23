package conversation_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// LeaveConversation gère la suppression côté client (MP) et le départ (Groupe/Communauté)
func LeaveConversation(ctx context.Context, callerID int64, convID int64, input conversation_models.LeaveConversationInput) error {
	// 1. SÉCURITÉ ET RÉCUPÉRATION (Objet Complet)
	// On s'assure que le caller fait bien partie de la conversation
	mem, err := security_service.LeftMember(ctx, convID, callerID)
	if err != nil {
		return err
	}
	conv, err := object_cache_service.GetConversationFromObjectCache(ctx, convID)
	if err != nil {
		return errors.New("impossible de charger les détails de la conversation")
	}

	// 2. GESTION DU PROPRIÉTAIRE (Type > 0)
	// On vérifie en O(1) combien de personnes actives sont dans la conversation via Redis
	participantCount, _ := redis.ConvParticipants.SCard(ctx, convID)

	if conv.Type > 0 && mem.Role == 2 && participantCount > 1 {
		// Le propriétaire quitte mais il reste du monde : il DOIT nommer un nouvel admin
		if input.NewOwnerID == 0 {
			return errors.New("vous devez transférer la propriété à un administrateur avant de quitter")
		}

		// Vérification du nouveau propriétaire
		newOwnerMem, err := security_service.LeftMember(ctx, convID, input.NewOwnerID)
		if err != nil || newOwnerMem.Role != 1 {
			return errors.New("le nouveau propriétaire doit être un administrateur existant du groupe")
		}

		// Promotion du nouveau propriétaire
		newOwnerMem.Role = 2
		newOwnerMem.UpdatedAt = time.Now().UTC()
		_ = object_cache_service.SetMemberInObjectCache(ctx, newOwnerMem)
		_ = redis.EnqueueDB(ctx, newOwnerMem.ID, convID, redis.EntityMembers, redis.ActionUpdate, newOwnerMem, redis.TargetAll)
	}

	// 3. APPLICATION DU DÉPART (Rôle = -1)
	mem.Role = -1
	mem.UpdatedAt = time.Now().UTC()
	_ = object_cache_service.SetMemberInObjectCache(ctx, mem)

	// === NOUVEAU : PURGE SYNCHRONE DU SPEED CACHE ===
	_ = cache_service.RemoveMemberFromSpeedCache(ctx, mem.ConversationID, mem.UserID)

	// Write-behind...
	err = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)

	// Envoie notification (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), convID, "member.left", mem)
			if err != nil {
				_ = fmt.Errorf("Failed to broadcast member left: %v", err)
			}
		}()
	}

	// 4. VÉRIFICATION DE L'EXTINCTION DE LA CONVERSATION
	// Si on était le dernier participant (participantCount redescend à 0 après notre départ)
	if participantCount <= 1 {
		// On attribue le bon statut selon le type
		if conv.Type == 0 {
			conv.State = -1 // MP Supprimé
		} else {
			conv.State = -2 // Groupe/Communauté Supprimé
		}
		conv.UpdatedAt = time.Now().UTC()

		// Mise à jour L1 et File Asynchrone
		_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
		_ = redis.EnqueueDB(ctx, conv.ID, conv.ID, redis.EntityConversation, redis.ActionUpdate, conv, redis.TargetAll)
	}

	return nil
}
