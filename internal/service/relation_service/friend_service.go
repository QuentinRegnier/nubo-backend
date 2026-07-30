package relation_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// ToggleFriend gère les ajouts et retraits d'amis
func ToggleFriend(ctx context.Context, callerID int64, targetID int64, action string) error {
	if callerID == targetID {
		return errors.New("vous ne pouvez pas être ami avec vous-même")
	}

	// 1. Lire l'état actuel
	currentState := cache_service.RelationValue(ctx, targetID, callerID)

	if currentState == -1 {
		return errors.New("action impossible : utilisateur bloqué")
	}

	var newState int
	var dbAction redis.ActionType

	// 2. Logique de transition d'état
	if action == "friend" {
		if currentState == 2 {
			return nil // Déjà ami, coupe-circuit
		}
		newState = 2
		if currentState == 0 {
			dbAction = redis.ActionCreate // Pas encore abonné, on crée la relation directement
		} else { // currentState == 1
			dbAction = redis.ActionUpdate // Déjà abonné, on met à jour la relation
		}
	} else if action == "unfriend" {
		if currentState == 0 || currentState == 1 {
			return nil // N'est déjà pas/plus ami, coupe-circuit
		}
		// S'il était ami (2), il redevient un simple abonné (1)
		newState = 1
		dbAction = redis.ActionUpdate
	} else {
		return errors.New("action non reconnue")
	}

	// 3. Mise à jour immédiate du Cache L1
	if err := cache_service.UpdateRelationState(ctx, targetID, callerID, newState); err != nil {
		return err
	}

	// 4. Persistance Asynchrone
	now := time.Now().UTC()
	payload := relation_models.RelationPayload{
		ID:          pkg.GenerateID(), // ID virtuel, l'update BDD se fera sur primary/secondary ID !
		PrimaryID:   callerID,
		SecondaryID: targetID,
		State:       newState,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// PartitionKey = targetID pour assurer l'ordre chronologique des requêtes sur ce profil
	return redis.EnqueueDB(ctx, payload.ID, targetID, redis.EntityRelation, dbAction, payload, redis.TargetAll)
}
