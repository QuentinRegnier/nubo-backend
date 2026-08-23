package relation_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
)

// ToggleFollow gère l'abonnement et le désabonnement avec idempotence en RAM
func ToggleFollow(ctx context.Context, callerID int64, targetID int64, action string) error {
	if callerID == targetID {
		return errors.New("vous ne pouvez pas vous suivre vous-même")
	}

	// 1. Lire l'état actuel en cascade O(1) (0=Rien, 1=Follow, 2=Ami, -1=Banni)
	currentState := cache_service.RelationValue(ctx, targetID, callerID)

	// Sécurité absolue : Blocage
	if currentState == -1 {
		return errors.New("action impossible : utilisateur bloqué")
	}

	newState := currentState
	dbAction := redis.ActionCreate

	// 2. Déterminer la nouvelle action et bloquer les doublons (Spam Clic)
	if action == "follow" {
		if currentState == 1 || currentState == 2 {
			return nil // Déjà suivi ou ami, on coupe le circuit ici (Zéro I/O BDD)
		}
		newState = 1
		dbAction = redis.ActionCreate
	} else if action == "unfollow" {
		if currentState == 0 {
			return nil // Déjà rien, coupe-circuit
		}
		newState = 0
		dbAction = redis.ActionDelete
	} else {
		return errors.New("action non reconnue")
	}

	// 3. Mise à jour immédiate du Cache L1 (SpeedRelations & SpeedFollowers)
	// Cela impactera instantanément le rendu UI et les futurs Fan-Outs de posts
	if err := cache_service.UpdateRelationState(ctx, targetID, callerID, newState); err != nil {
		return err
	}

	// 4. Persistance asynchrone (Write-Behind vers L2 et L3)
	now := time.Now().UTC()
	payload := relation_models.RelationPayload{
		ID:          pkg.GenerateID(),
		PrimaryID:   callerID,
		SecondaryID: targetID,
		State:       newState,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// PartitionKey = targetID pour centraliser les requêtes sur le shard de la cible
	err := redis.EnqueueDB(ctx, payload.ID, targetID, redis.EntityRelation, dbAction, payload, redis.TargetAll)

	if err == nil && newState == 1 && currentState == 0 {
		go func() {
			err := notification_service.DispatchNotification(context.Background(), targetID, callerID, "relation_followed", callerID)
			if err != nil {
				_ = fmt.Errorf("ToggleFollow: failed to dispatch notification for follow from %d to %d: %v", callerID, targetID, err)
			}
		}()
	}

	return err
}
