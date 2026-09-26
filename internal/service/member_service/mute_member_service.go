package member_service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RESTRICTION TEMPORELLE (MUTE) D'UN MEMBRE
// ############################################################################

// MuteMember gère l'interdiction de parler pour un membre.
func MuteMember(ctx context.Context, callerID int64, input member_models.MuteMemberInput) error {

	// ── ÉTAPE 1 : CONTRÔLE D'ACCÈS ET HIERARCHIE ────────────────────────────

	callerMemberPayload, errCaller := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if errCaller != nil {
		return errCaller
	}
	if callerMemberPayload.Role < variables.MemberRoleAdmin {
		return nubo_error.NewForbidden(nubo_error.CodeForbidden, "Seuls les administrateurs peuvent appliquer une restriction à un membre.", nil)
	}

	targetMemberPayload, errTarget := security_service.LeftMember(ctx, input.ConversationID, input.TargetUserID)
	if errTarget != nil {
		return errTarget
	}

	// Un administrateur ne peut pas muter un autre administrateur ou un propriétaire.
	if targetMemberPayload.Role >= callerMemberPayload.Role {
		return nubo_error.NewForbidden(nubo_error.CodeForbidden, "Vous ne pouvez pas restreindre un membre de rang égal ou supérieur au vôtre.", nil)
	}

	// ── ÉTAPE 2 : LOGIQUE ALGORITHMIQUE ET CASCADE (L1 -> L2 -> L3) ─────────
	// Objectif : Ne pas muter quelqu'un qui n'a déjà pas le droit de parler selon les lois du groupe.

	var conversationPayload conversation_models.ConversationPayload

	conversationPayload, errCache := object_cache_service.GetConversationFromObjectCache(ctx, input.ConversationID)

	if errCache != nil || conversationPayload.ID == 0 {
		var errMongo error
		conversationPayload, errMongo = mongo.MongoGetConversation(input.ConversationID)

		if errMongo == nil && conversationPayload.ID != 0 {
			// AUTO-GUÉRISON L2 -> L1
			_ = object_cache_service.SetConversationInObjectCache(ctx, conversationPayload)
		} else {
			// FALLBACK L3 (PostgreSQL)
			var errPg error
			conversationPayload, errPg = postgres.FuncGetConversation(ctx, input.ConversationID)
			if errPg != nil {
				logger.Log.Error().Err(errPg).Int64("conv_id", input.ConversationID).Msg("Échec de la récupération L3 de la conversation pour le Mute")
				return nubo_error.NewInternal()
			}

			if conversationPayload.ID != 0 {
				// AUTO-GUÉRISON L3 -> L2 & L1
				go func(c conversation_models.ConversationPayload) {
					bgCtx := context.Background()
					_ = redis.EnqueueDB(bgCtx, c.ID, c.ID, redis.EntityConversation, redis.ActionUpdate, c, redis.TargetMongo)
				}(conversationPayload)

				_ = object_cache_service.SetConversationInObjectCache(ctx, conversationPayload)
			}
		}
	}

	if conversationPayload.ID == 0 {
		return nubo_error.NewNotFound(nubo_error.CodeNotFound, "Conversation introuvable ou inactive.", nil)
	}

	// Règle métier : Si le groupe est en mode "Admins Uniquement" (WritePermission = 1),
	// le membre standard n'a déjà pas le droit de parole, la sanction est inutile.
	if conversationPayload.Settings.WritePermission == 1 && targetMemberPayload.Role == variables.MemberRoleNormal {
		return nubo_error.NewForbidden(nubo_error.CodeForbidden, "Ce membre n'a déjà pas le droit de parole en raison des permissions actuelles du groupe.", nil)
	}

	// ── ÉTAPE 3 : APPLICATION DE LA PUNITION ────────────────────────────────

	targetMemberPayload.Settings.RestrictedUntil = input.RestrictedUntil
	targetMemberPayload.UpdatedAt = domain.NowMillis()

	// ── ÉTAPE 4 : MISE À JOUR SYNCHRONE (L1 OBJECT ET SPEED CACHE) ──────────

	_ = object_cache_service.SetMemberInObjectCache(ctx, targetMemberPayload)

	memberCompositeID := fmt.Sprintf("%d:%d", input.ConversationID, input.TargetUserID)
	var liteMember lite_models.MemberLiteRequest

	if errSpeedCache := redis.ConvMembers.GetObject(ctx, memberCompositeID, &liteMember); errSpeedCache == nil {
		liteMember.Settings.RestrictedUntil = input.RestrictedUntil
		_ = redis.ConvMembers.SetObject(ctx, memberCompositeID, liteMember)
	}

	// ── ÉTAPE 5 : PERSISTANCE ASYNCHRONE ET LEDGER (WRITE-BEHIND) ───────────

	errQueue := redis.EnqueueDB(ctx, targetMemberPayload.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, targetMemberPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("user_id", targetMemberPayload.UserID).Msg("Échec du Write-Behind pour la restriction d'un membre")
		return nubo_error.NewInternal()
	}

	// SYNC LEDGER (Trigger d'invalidation mutuelle)
	go func(cID int64) {
		bgCtx := context.Background()
		participantsStringList, _ := redis.ConvParticipants.SMembers(bgCtx, cID)

		var syncTargetIDs []int64
		for _, pStr := range participantsStringList {
			if id, errParse := strconv.ParseInt(pStr, 10, 64); errParse == nil {
				syncTargetIDs = append(syncTargetIDs, id)
			}
		}
		_ = cache_service.RecordConversationMutation(bgCtx, cID, syncTargetIDs)
	}(input.ConversationID)

	// ── ÉTAPE 6 : NOTIFICATION ET MESSAGE SYSTÈME ───────────────────────────

	go func() {
		bgCtx := context.Background()

		// A. Diffusion temps réel de la pénalité pour l'interface UI
		_ = realtime_service.BroadcastToConversation(bgCtx, input.ConversationID, "member.muted", targetMemberPayload)

		// B. Message Système pour historique
		systemMessageInput := message_models.CreateMessageInput{
			ConversationID: input.ConversationID,
			MessageType:    variables.MessageTypeSystem,
			Content:        "Un membre a été restreint.",
			Attachments: map[string]any{
				"event_type":       "member_muted",
				"target_user_id":   input.TargetUserID,
				"restricted_until": input.RestrictedUntil,
			},
		}

		// By-pass du mode lecture-seule pour l'Admin s'il était lui-même restreint (isInternal = true)
		_, _ = message_service.CreateMessage(bgCtx, callerID, input.ConversationID, systemMessageInput, true)
	}()

	return nil
}
