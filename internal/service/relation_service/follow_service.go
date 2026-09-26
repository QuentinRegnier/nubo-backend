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
// # SERVICE : ABONNEMENT (FOLLOW)
// ############################################################################

// ToggleFollow gère l'abonnement et le désabonnement avec idempotence en RAM.
func ToggleFollow(ctx context.Context, callerID int64, targetID int64, requestedAction string) error {

	// ── ÉTAPE 1 : RÈGLE MÉTIER (AUTO-FOLLOW INTERDIT) ───────────────────────
	if callerID == targetID {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Vous ne pouvez pas vous abonner à vous-même.", nil)
	}

	// ── ÉTAPE 2 : LECTURE DE L'ÉTAT ACTUEL (O(1) RAM L1) ────────────────────
	currentRelationState := cache_service.RelationValue(ctx, targetID, callerID)

	// Sécurité absolue : Bloqué
	if currentRelationState == variables.RelationStateBlocked {
		return nubo_error.NewForbidden(nubo_error.CodeForbidden, "Action impossible : Utilisateur bloqué.", nil)
	}

	newRelationState := currentRelationState
	redisActionType := redis.ActionCreate

	// ── ÉTAPE 3 : LOGIQUE DE TRANSITION ET IDEMPOTENCE (SPAM CLIC) ──────────
	if requestedAction == "follow" {
		if currentRelationState == variables.RelationStateFollow || currentRelationState == variables.RelationStateFriend {
			return nil // Déjà suivi ou ami, coupe-circuit instantané (Zéro I/O BDD).
		}
		newRelationState = variables.RelationStateFollow
		redisActionType = redis.ActionCreate

	} else if requestedAction == "unfollow" {
		if currentRelationState == variables.RelationStateNone {
			return nil // Déjà aucun lien, coupe-circuit.
		}
		newRelationState = variables.RelationStateNone
		redisActionType = redis.ActionDelete

	} else {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Action de relation non reconnue.", nil)
	}

	// ── ÉTAPE 4 : MISE À JOUR IMMÉDIATE DU CACHE L1 ─────────────────────────
	// Impacte instantanément le rendu UI et les futurs Fan-Outs de posts.
	currentTime := time.Now().UTC()
	errCache := cache_service.UpdateRelationState(ctx, targetID, callerID, newRelationState, domain.TimeToMillis(currentTime))
	if errCache != nil {
		logger.Log.Error().Err(errCache).Msg("Échec de la mise à jour du Cache L1 lors d'un ToggleFollow")
		return nubo_error.NewInternal()
	}

	// ── ÉTAPE 5 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────
	relationPayload := relation_models.RelationPayload{
		ID:          pkg.GenerateID(),
		PrimaryID:   callerID,
		SecondaryID: targetID,
		State:       newRelationState,
		CreatedAt:   domain.TimeToMillis(currentTime),
		UpdatedAt:   domain.TimeToMillis(currentTime),
	}

	// PartitionKey = targetID pour centraliser les requêtes sur le shard de la cible.
	errQueue := redis.EnqueueDB(ctx, relationPayload.ID, targetID, redis.EntityRelation, redisActionType, relationPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("target_id", targetID).Msg("Échec du Write-Behind pour ToggleFollow")
		return nubo_error.NewInternal()
	}

	// ── ÉTAPE 6 : DISTRIBUTION DES NOTIFICATIONS ────────────────────────────
	if newRelationState == variables.RelationStateFollow && currentRelationState == variables.RelationStateNone {
		go func() {
			backgroundCtx := context.Background()
			errNotif := notification_service.DispatchNotification(backgroundCtx, targetID, callerID, variables.EventRelationFollowed, callerID)
			if errNotif != nil {
				logger.Log.Error().Err(errNotif).Msg("Échec de l'expédition de la notification pour Follow")
			}
		}()
	}

	return nil
}
