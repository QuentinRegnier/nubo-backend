package conversation_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UnpinConversation gère le retrait d'une épingle sur une conversation.
func UnpinConversation(ctx context.Context, callerID int64, input conversation_models.UnpinConversationInput) error {
	// 1. Récupération sécurisée du membre (Cascade L1->L2->L3)
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return err
	}

	// 2. Idempotence : Si déjà désépinglée (-1), on s'arrête silencieusement
	if mem.Settings.Pinned == -1 {
		return nil
	}

	// 3. Application du retrait
	mem.Settings.Pinned = -1
	mem.UpdatedAt = time.Now().UTC()

	// 4. Mise à jour synchrone L1 (Object et Speed Cache)
	_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
	_ = cache_service.UpdateMemberSpeedCache(ctx, lite_models.MemberLiteRequest{
		ConversationID:  mem.ConversationID,
		UserID:          mem.UserID,
		Role:            mem.Role,
		Settings:        service.ToMemberSettingsLite(mem.Settings),
		UnreadCount:     mem.UnreadCount,
		FrozenMessageID: mem.FrozenMessageID,
		JoinedAt:        mem.JoinedAt.UnixMilli(),
	})

	// 5. Persistance Asynchrone (Write-Behind)
	// La clé de partition est l'ID de la conversation pour conserver l'ordre des requêtes
	return redis.EnqueueDB(ctx, mem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)
}
