package conversation_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

func TogglePinConversation(ctx context.Context, callerID int64, input conversation_models.PinConversationInput) (conversation_models.PinConversationOutput, error) {
	// 1. Récupération sécurisée du membre
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return conversation_models.PinConversationOutput{}, err
	}

	if mem.Settings.Pinned >= 0 {
		return conversation_models.PinConversationOutput{}, nil // Déjà épinglé, on garantit l'idempotence
	}

	// A. Récupération des indices utilisés (L2 -> L3)
	var indices []int
	indices, errIdx := mongo.MongoGetPinnedIndices(callerID)
	if errIdx != nil || len(indices) == 0 {
		indices, _ = postgres.FuncGetPinnedIndices(ctx, callerID)
	}

	// B. Limite stricte
	if len(indices) >= 3 {
		return conversation_models.PinConversationOutput{}, nubo_error.NewBadRequest("MAX_PIN_REACHED", "Vous ne pouvez épingler que 3 conversations.", nil)
	}

	// C. Détermination du slot vide (0, 1 ou 2)
	used := make(map[int]bool)
	for _, idx := range indices {
		used[idx] = true
	}

	availableIdx := -1
	for i := 0; i < 3; i++ {
		if !used[i] {
			availableIdx = i
			break
		}
	}
	mem.Settings.Pinned = availableIdx

	mem.UpdatedAt = time.Now().UTC()

	// 3. Mise à jour synchrone L1 (Object et Speed Cache)
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

	// 4. Persistance (Write-Behind)

	output := conversation_models.PinConversationOutput{}

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé TOUT À LA FIN de la fonction. Cela écrase tout timestamp qui aurait
	// pu être généré précédemment (par ex. à l'intérieur de AddMembersToConversation)
	// et garantit que le client reçoit la date de la fin absolue de la transaction.
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	output.InboxUpdateAt = time.UnixMilli(timestampMs)

	return output, redis.EnqueueDB(ctx, mem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)
}
