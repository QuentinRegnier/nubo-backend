package conversation_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DES CURSEURS DE LECTURE (WATERMARKS)
// ############################################################################

// GetWatermarks récupère les curseurs de lecture (LastReadMessageID) en O(1)
// depuis la RAM pour tous les participants d'une conversation donnée.
func GetWatermarks(ctx context.Context, callerID int64, input conversation_models.GetWatermarksInput) (conversation_models.GetWatermarksOutput, error) {

	// ── ÉTAPE 1 : CONTRÔLE D'ACCÈS ZERO-TRUST ────────────────────────────────
	callerMemberPayload, errSecurity := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if errSecurity != nil || callerMemberPayload.Role < variables.MemberRoleNormal {
		return conversation_models.GetWatermarksOutput{}, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Vous n'êtes pas membre de cette conversation.", errSecurity)
	}

	// ── ÉTAPE 2 : RÉCUPÉRATION O(1) DEPUIS LE SPEED CACHE L1 ─────────────────
	rawWatermarksMap, errCache := cache_service.GetWatermarksFromSpeedCache(ctx, input.ConversationID)
	if errCache != nil {
		// Erreur interne Redis masquée sous une AppError standardisée
		return conversation_models.GetWatermarksOutput{}, nubo_error.NewInternal()
	}

	// ── ÉTAPE 3 : SÉRIALISATION DES CLÉS EN CHAÎNES POUR LE JSON ─────────────
	formattedWatermarks := make(map[string]int64, len(rawWatermarksMap))
	for participantUserID, lastReadMessageID := range rawWatermarksMap {
		formattedWatermarks[strconv.FormatInt(participantUserID, 10)] = lastReadMessageID
	}

	return conversation_models.GetWatermarksOutput{
		Watermarks: formattedWatermarks,
	}, nil
}
