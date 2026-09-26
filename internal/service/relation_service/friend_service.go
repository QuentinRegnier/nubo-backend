package relation_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : AMITIÉ (FRIENDS)
// ############################################################################

// ToggleFriend gère les ajouts et retraits d'amis entre utilisateurs.
func ToggleFriend(ctx context.Context, callerID int64, targetID int64, requestedAction string) error {

	// ── ÉTAPE 1 : RÈGLE MÉTIER (AUTO-AMITIÉ INTERDITE) ──────────────────────
	if callerID == targetID {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Vous ne pouvez pas être ami avec vous-même.", nil)
	}

	// ── ÉTAPE 2 : LECTURE DE L'ÉTAT ACTUEL (O(1) RAM L1) ────────────────────
	currentRelationState := cache_service.RelationValue(ctx, targetID, callerID)

	if currentRelationState == variables.RelationStateBlocked {
		return nubo_error.NewForbidden(nubo_error.CodeForbidden, "Action impossible : Utilisateur bloqué.", nil)
	}

	var newRelationState int
	var redisActionType redis.ActionType

	// ── ÉTAPE 3 : LOGIQUE DE TRANSITION ET IDEMPOTENCE ──────────────────────
	if requestedAction == "friend" {
		if currentRelationState == variables.RelationStateFriend {
			return nil // Déjà ami, coupe-circuit.
		}

		newRelationState = variables.RelationStateFriend

		if currentRelationState == variables.RelationStateNone {
			redisActionType = redis.ActionCreate // Pas encore abonné, on crée la relation d'amitié directement.
		} else {
			redisActionType = redis.ActionUpdate // Déjà abonné, on promeut la relation.
		}

	} else if requestedAction == "unfriend" {
		if currentRelationState == variables.RelationStateNone || currentRelationState == variables.RelationStateFollow {
			return nil // N'est déjà pas/plus ami, coupe-circuit.
		}

		// S'il était ami (2), le retrait le déclasse en simple abonné (1).
		newRelationState = variables.RelationStateFollow
		redisActionType = redis.ActionUpdate

	} else {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Action de relation non reconnue.", nil)
	}

	// ── ÉTAPE 4 : MISE À JOUR IMMÉDIATE DU CACHE L1 ─────────────────────────
	currentTime := time.Now().UTC()
	errCache := cache_service.UpdateRelationState(ctx, callerID, targetID, newRelationState, domain.TimeToMillis(currentTime))
	if errCache != nil {
		logger.Log.Error().Err(errCache).Msg("Échec de la mise à jour du Cache L1 lors d'un ToggleFriend")
		return nubo_error.NewInternal()
	}

	// ── ÉTAPE 5 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────
	relationPayload := relation_models.RelationPayload{
		ID:          pkg.GenerateID(), // ID virtuel, l'update BDD se fera sur PrimaryID / SecondaryID.
		PrimaryID:   targetID,
		SecondaryID: callerID,
		State:       newRelationState,
		CreatedAt:   domain.TimeToMillis(currentTime),
		UpdatedAt:   domain.TimeToMillis(currentTime),
	}

	// PartitionKey = targetID pour assurer l'ordre chronologique des requêtes.
	errQueue := redis.EnqueueDB(ctx, relationPayload.ID, targetID, redis.EntityRelation, redisActionType, relationPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("target_id", targetID).Msg("Échec du Write-Behind pour ToggleFriend")
		return nubo_error.NewInternal()
	}

	// ── ÉTAPE 6 : DISTRIBUTION DES NOTIFICATIONS ────────────────────────────
	if requestedAction == "friend" {
		go func() {
			backgroundCtx := context.Background()
			errNotif := notification_service.DispatchNotification(backgroundCtx, targetID, callerID, variables.EventFriendshipEst, callerID)
			if errNotif != nil {
				logger.Log.Error().Err(errNotif).Msg("Échec de l'expédition de la notification d'amitié")
			}
		}()
	}

	return nil
}
