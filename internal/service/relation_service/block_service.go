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
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : BLOCAGE D'UTILISATEURS
// ############################################################################

// ToggleBlock gère le blocage et le déblocage d'un utilisateur.
func ToggleBlock(ctx context.Context, callerID int64, targetID int64, requestedAction string) error {

	// ── ÉTAPE 1 : RÈGLE MÉTIER (AUTO-BLOCAGE INTERDIT) ──────────────────────
	if callerID == targetID {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Vous ne pouvez pas vous bloquer vous-même.", nil)
	}

	// ── ÉTAPE 2 : LECTURE DE L'ÉTAT ACTUEL (O(1) RAM L1) ────────────────────
	currentRelationState := cache_service.RelationValue(ctx, targetID, callerID)

	var newRelationState int
	var redisActionType redis.ActionType

	// ── ÉTAPE 3 : LOGIQUE DE TRANSITION D'ÉTAT ──────────────────────────────
	if requestedAction == "block" {
		if currentRelationState == variables.RelationStateBlocked {
			return nil // Idempotence : L'utilisateur est déjà bloqué.
		}

		newRelationState = variables.RelationStateBlocked
		if currentRelationState == variables.RelationStateNone {
			redisActionType = redis.ActionCreate // Création pure d'une relation de blocage
		} else {
			redisActionType = redis.ActionUpdate // Écrasement d'une amitié/abonnement par un blocage
		}
	} else if requestedAction == "unblock" {
		if currentRelationState != variables.RelationStateBlocked {
			return nil // Idempotence : L'utilisateur n'était pas bloqué.
		}

		newRelationState = variables.RelationStateNone
		redisActionType = redis.ActionDelete // Suppression physique de la relation
	} else {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Action de relation non reconnue.", nil)
	}

	// ── ÉTAPE 4 : MISE À JOUR IMMÉDIATE DU CACHE L1 ─────────────────────────
	// Retire automatiquement des Followers/Friends si le nouvel état est -1
	currentTime := time.Now().UTC()
	errCache := cache_service.UpdateRelationState(ctx, targetID, callerID, newRelationState, domain.TimeToMillis(currentTime))
	if errCache != nil {
		logger.Log.Error().Err(errCache).Msg("Échec de la mise à jour du Cache L1 lors d'un ToggleBlock")
		return nubo_error.NewInternal()
	}

	// ── ÉTAPE 5 : PURGE DES RECOMMANDATIONS (CUCKOO & FEEDS) ────────────────
	// Si on bloque, on force l'algorithme à oublier ce qu'il a généré et
	// à exclure l'utilisateur bloqué au prochain Swipe.
	if requestedAction == "block" {
		service.ResetCuckooFilter(ctx, callerID)
		service.ResetCuckooFilter(ctx, targetID)

		_ = redis.FeedsPersonalized.DeleteObject(ctx, callerID)
		_ = redis.FeedsPersonalized.DeleteObject(ctx, targetID)
	}

	// ── ÉTAPE 6 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────
	relationPayload := relation_models.RelationPayload{
		ID:          pkg.GenerateID(),
		PrimaryID:   callerID,
		SecondaryID: targetID,
		State:       newRelationState,
		CreatedAt:   domain.TimeToMillis(currentTime),
		UpdatedAt:   domain.TimeToMillis(currentTime),
	}

	// PartitionKey = targetID pour assurer l'ordre chronologique des requêtes sur le profil cible
	errQueue := redis.EnqueueDB(ctx, relationPayload.ID, targetID, redis.EntityRelation, redisActionType, relationPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("target_id", targetID).Msg("Échec du Write-Behind pour ToggleBlock")
		return nubo_error.NewInternal()
	}

	return nil
}
