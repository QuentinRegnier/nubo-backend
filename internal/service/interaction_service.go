package service

import (
	"context"
	"sync"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

type Interaction struct {
	ActorID   int64
	TargetID  int64
	Type      string
	Timestamp int64
}

var (
	interMutex        sync.Mutex
	interactionBuffer []Interaction
)

func init() {
	go flushInteractionsPeriodically()
}

func RegisterLike(actorID, postID int64) {
	interMutex.Lock()
	defer interMutex.Unlock()

	interactionBuffer = append(interactionBuffer, Interaction{
		ActorID:   actorID,
		TargetID:  postID,
		Type:      "like",
		Timestamp: time.Now().Unix(),
	})
}

func RegisterView(actorID, postID int64) {
	interMutex.Lock()
	defer interMutex.Unlock()

	interactionBuffer = append(interactionBuffer, Interaction{
		ActorID:   actorID,
		TargetID:  postID,
		Type:      "view",
		Timestamp: time.Now().Unix(),
	})
}

func flushInteractionsPeriodically() {
	ticker := time.NewTicker(5 * time.Second)
	ctx := context.Background()

	for range ticker.C {
		interMutex.Lock()
		if len(interactionBuffer) == 0 {
			interMutex.Unlock()
			continue
		}

		batchToProcess := interactionBuffer
		interactionBuffer = make([]Interaction, 0)
		interMutex.Unlock()

		processCacheUpdates(ctx, batchToProcess)
	}
}

func processCacheUpdates(ctx context.Context, batch []Interaction) {
	likesToAdd := make(map[int64]int)
	viewsToAdd := make(map[int64]int)

	// 1. Agrégation par type
	for _, interaction := range batch {
		if interaction.Type == "like" {
			likesToAdd[interaction.TargetID]++
		} else if interaction.Type == "view" {
			viewsToAdd[interaction.TargetID]++
		}
	}

	var eventsToQueue []redis.AsyncEvent

	// 2. Traitement mutualisé pour éviter la duplication de code
	eventsToQueue = append(eventsToQueue, processMetricBatch(ctx, likesToAdd, "like")...)
	eventsToQueue = append(eventsToQueue, processMetricBatch(ctx, viewsToAdd, "view")...)

	// 3. Envoi vers le REQUEST Cache
	if len(eventsToQueue) > 0 {
		// TODO: Implémenter l'envoi du tableau d'événements 'eventsToQueue' dans le REQUEST Cache
		_ = eventsToQueue
	}
}

// processMetricBatch mutualise la logique de mise à jour pour les likes, vues, etc.
func processMetricBatch(ctx context.Context, items map[int64]int, metricType string) []redis.AsyncEvent {
	var events []redis.AsyncEvent

	for postID, count := range items {
		alreadyEvaluated := false
		var postObj domain.PostRequest

		// Si l'objet est en RAM, on met à jour l'OBJECT Cache et le MOST Cache instantanément
		if err := redis.Posts.GetObject(ctx, postID, &postObj); err == nil && postObj.ID != 0 {
			var newTotal float64

			if metricType == "like" {
				postObj.LikeCount += count
				newTotal = float64(postObj.LikeCount)
				EvaluatePostAfterLike(ctx, postID, newTotal, postObj.Hashtags)
			} else if metricType == "view" {
				postObj.ViewCount += count
				newTotal = float64(postObj.ViewCount)
				EvaluatePostAfterView(ctx, postID, newTotal, postObj.Hashtags)
			}

			_ = redis.Posts.SetObject(ctx, postID, postObj)
			alreadyEvaluated = true
		}

		// Détermination du type d'entité pour le Worker (Postgres/Mongo)
		var entityType redis.EntityType
		if metricType == "like" {
			entityType = redis.EntityLike
		} else {
			entityType = redis.EntityView
		}

		// Préparation de la tâche pour le REQUEST Cache
		events = append(events, redis.AsyncEvent{
			Type:   entityType,
			Action: redis.ActionCreate,
			Payload: map[string]interface{}{
				"target_id":               postID,
				"count":                   count,
				"already_evaluated_redis": alreadyEvaluated,
			},
		})
	}

	return events
}
