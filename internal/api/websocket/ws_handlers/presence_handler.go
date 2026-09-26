package ws_handlers

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// =========================================================================
// CAS A : VIEWPORT SYNC (INBOX / LISTE)
// =========================================================================

// HandleSyncPresence traite la demande de synchronisation de présence (Cas A)
func HandleSyncPresence(ctx context.Context, rawPayload []byte) (any, error) {
	var input sync_models.SyncPresenceInput

	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nil, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}

	if err := pkg.ValidateStruct(&input); err != nil {
		return nil, nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation échouée.", err)
	}

	presenceMap, err := cache_service.AreUsersOnline(ctx, input.UserIDs)
	if err != nil {
		return nil, err
	}

	return presenceMap, nil
}

// =========================================================================
// CAS B : FOCUS CONVERSATION (INSTANTANÉITÉ EN DM / GROUPE ACTIF)
// =========================================================================

// HandleFocusConversation gère l'arrivée/départ d'un utilisateur sur un écran de chat actif (Cas B).
// Diffuse instantanément "user.presence" aux seuls participants de cette conversation.
func HandleFocusConversation(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input sync_models.FocusConversationInput

	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}

	if err := pkg.ValidateStruct(&input); err != nil {
		return nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation échouée.", err)
	}

	// 1. SÉCURITÉ ZERO-TRUST : On vérifie que l'utilisateur est bien membre de la conversation
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil || mem.Role < 0 {
		return nubo_error.NewForbidden("ACCESS_DENIED", "Accès refusé.", err)
	}

	// 2. Événement diffusé en direct aux membres de la conversation
	// Si input.Active == true, on prévient User A que User B vient d'ouvrir le chat (online immédiat)
	// Si input.Active == false, on prévient User A que User B n'a plus les yeux sur la discussion
	broadcastPayload := map[string]any{
		"conversation_id": input.ConversationID,
		"user_id":         callerID,
		"is_online":       input.Active,
	}

	return realtime_service.BroadcastToConversation(ctx, input.ConversationID, "user.presence", broadcastPayload)
}
