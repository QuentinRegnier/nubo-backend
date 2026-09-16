package relation_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// ToggleBlock gère le blocage et le déblocage d'un utilisateur
func ToggleBlock(ctx context.Context, callerID int64, targetID int64, action string) error {
	if callerID == targetID {
		return errors.New("vous ne pouvez pas vous bloquer vous-même")
	}

	// 1. Lire l'état actuel
	currentState := cache_service.RelationValue(ctx, targetID, callerID)

	var newState int
	var dbAction redis.ActionType

	// 2. Logique de transition d'état
	if action == "block" {
		if currentState == -1 {
			return nil // Déjà bloqué, coupe-circuit
		}
		newState = -1
		if currentState == 0 {
			dbAction = redis.ActionCreate // Création d'une relation de blocage
		} else {
			dbAction = redis.ActionUpdate // Écrasement de l'amitié/abonnement par un blocage
		}
	} else if action == "unblock" {
		if currentState != -1 {
			return nil // N'était pas bloqué, coupe-circuit
		}
		newState = 0
		dbAction = redis.ActionDelete // Suppression physique de la relation en BDD
	} else {
		return errors.New("action non reconnue")
	}

	// 3. Mise à jour immédiate du Cache L1 (Retire automatiquement des Followers/Friends si newState = -1)
	if err := cache_service.UpdateRelationState(ctx, targetID, callerID, newState); err != nil {
		return err
	}

	// 4. Purge mutuelle des Cuckoo Filters et des Feed personnalisés (Cas du Blocage)
	// Force l'algorithme à oublier ce qu'il a généré et à exclure l'utilisateur bloqué au prochain Swipe
	if action == "block" {
		service.ResetCuckooFilter(ctx, callerID)
		service.ResetCuckooFilter(ctx, targetID)
		_ = redis.FeedsPersonalized.DeleteObject(ctx, callerID)
		_ = redis.FeedsPersonalized.DeleteObject(ctx, targetID)
	}

	// 5. Persistance Asynchrone
	now := time.Now().UTC()
	payload := relation_models.RelationPayload{
		ID:          pkg.GenerateID(),
		PrimaryID:   callerID,
		SecondaryID: targetID,
		State:       newState,
		CreatedAt:   domain.TimeToMillis(now),
		UpdatedAt:   domain.TimeToMillis(now),
	}

	// PartitionKey = targetID pour assurer l'ordre chronologique des requêtes sur ce profil
	return redis.EnqueueDB(ctx, payload.ID, targetID, redis.EntityRelation, dbAction, payload, redis.TargetAll)
}
