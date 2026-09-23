package conversation_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// GetWatermarks récupère les curseurs de lecture (O(1) L1 Cache) de tous les participants d'une conversation.
func GetWatermarks(ctx context.Context, callerID int64, input conversation_models.GetWatermarksInput) (conversation_models.GetWatermarksOutput, error) {
	// 1. SÉCURITÉ ZERO-TRUST : L'utilisateur doit être membre de la conversation
	_, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return conversation_models.GetWatermarksOutput{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Vous n'êtes pas membre de cette conversation.", err)
	}

	// 2. RÉCUPÉRATION O(1) DEPUIS LE CACHE L1 (Le service de cache a été défini à l'étape 1.2)
	rawWatermarks, errCache := cache_service.GetWatermarksFromSpeedCache(ctx, input.ConversationID)
	if errCache != nil {
		return conversation_models.GetWatermarksOutput{}, nubo_error.NewInternal(errCache)
	}

	// 3. MAPPING DYNAMIQUE : Conversion de map[int64]int64 vers map[string]int64 pour la sortie JSON
	watermarksStr := make(map[string]int64)
	for uID, mID := range rawWatermarks {
		watermarksStr[strconv.FormatInt(uID, 10)] = mID
	}

	return conversation_models.GetWatermarksOutput{
		Watermarks: watermarksStr,
	}, nil
}
