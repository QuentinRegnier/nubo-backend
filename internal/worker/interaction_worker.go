package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

type Interaction struct {
	ActorID   int64
	TargetID  int64
	Type      string
	Timestamp int64
}

var (
	// Canal asynchrone bufferisé (50 000 emplacements).
	// Amortit les Thundering Herds (pics soudains de trafic) sans bloquer les requêtes HTTP.
	interactionChan = make(chan Interaction, 50000)
)

func init() {
	go flushInteractionsPeriodically()
}

// RegisterView met en file d'attente une incrémentation de vue qualitative
func RegisterView(actorID int64, postID int64) {
	select {
	case interactionChan <- Interaction{
		ActorID:   actorID,
		TargetID:  postID,
		Type:      "view",
		Timestamp: time.Now().Unix(),
	}:
	default:
		// BACKPRESSURE
	}
}

// RegisterUnread met en file d'attente une incrémentation de message non lu
func RegisterUnread(convID int64, userID int64) {
	select {
	case interactionChan <- Interaction{
		ActorID:   userID, // Celui qui reçoit l'Unread
		TargetID:  convID, // La conversation concernée
		Type:      "unread",
		Timestamp: time.Now().Unix(),
	}:
	default:
		// BACKPRESSURE
	}
}

func flushInteractionsPeriodically() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ctx := context.Background()
	batch := make([]Interaction, 0, 5000)

	for {
		select {
		case interaction := <-interactionChan:
			batch = append(batch, interaction)

			// Vidage immédiat en cas de viralité extrême
			if len(batch) >= 5000 {
				processCacheUpdates(ctx, batch)
				batch = make([]Interaction, 0, 5000)
			}
		case <-ticker.C:
			// Toutes les 5 secondes, on vide le tampon
			if len(batch) > 0 {
				processCacheUpdates(ctx, batch)
				batch = make([]Interaction, 0, 5000)
			}
		}
	}
}

// processCacheUpdates agrège et expédie les paquets de compteurs vers les BDD
func processCacheUpdates(ctx context.Context, batch []Interaction) {
	viewsToAdd := make(map[int64]int)
	unreadsToAdd := make(map[string]int) // Clé = "convID:userID"

	// 1. Agrégation mathématique en RAM
	for _, interaction := range batch {
		if interaction.Type == "view" {
			viewsToAdd[interaction.TargetID]++
		} else if interaction.Type == "unread" {
			key := fmt.Sprintf("%d:%d", interaction.TargetID, interaction.ActorID)
			unreadsToAdd[key]++
		}
	}

	var eventsToQueue []redis.AsyncEvent

	// 2. Traitement des Vues
	for postID, count := range viewsToAdd {
		eventsToQueue = append(eventsToQueue, redis.AsyncEvent{
			Type:   redis.EntityView,
			Action: redis.ActionCreate,
			Payload: map[string]interface{}{
				"target_id": postID,
				"count":     count, // On flag la valeur absolue du paquet
			},
			Targets: redis.TargetPostgres | redis.TargetMongo,
		})
	}

	// 3. Traitement des Unreads
	for key, count := range unreadsToAdd {
		var convID, userID int64
		_, err := fmt.Sscanf(key, "%d:%d", &convID, &userID)
		if err != nil {
			return
		}

		eventsToQueue = append(eventsToQueue, redis.AsyncEvent{
			Type:   redis.EntityMembers,
			Action: redis.ActionUpdate,
			Payload: map[string]interface{}{
				"conversation_id": convID,
				"user_id":         userID,
				"unread_delta":    count, // On passe un DELTA
			},
			Targets: redis.TargetPostgres | redis.TargetMongo,
		})
	}

	// 4. Envoi sur la File Redis (Write-Behind)
	for _, event := range eventsToQueue {
		payloadMap, ok := event.Payload.(map[string]interface{})
		if !ok {
			continue
		}

		var partitionKey int64
		if event.Type == redis.EntityMembers {
			partitionKey = payloadMap["conversation_id"].(int64)
		} else {
			partitionKey = payloadMap["target_id"].(int64)
		}

		err := redis.EnqueueDB(
			ctx,
			event.ID,
			partitionKey,
			event.Type,
			event.Action,
			event.Payload,
			event.Targets,
		)
		if err != nil {
			logger.Log.Error().Err(err).Interface("event_type", event.Type).Msg("Interaction Worker : Impossible d'enqueue l'événement")
		}
	}
}
