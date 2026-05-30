package feed

import (
	"context"
	"time"

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
	// Amortit les Thundering Herds (pics soudains de trafic) sans bloquer les workers HTTP.
	interactionChan = make(chan Interaction, 50000)
)

func init() {
	go flushInteractionsPeriodically()
}

func RegisterLike(actorID, postID int64) {
	select {
	case interactionChan <- Interaction{
		ActorID:   actorID,
		TargetID:  postID,
		Type:      "like",
		Timestamp: time.Now().Unix(),
	}:
	default:
		// BACKPRESSURE : Si le buffer est saturé, on lâche l'interaction.
		// Règle d'or : Mieux vaut perdre un "Like" statistique que de crasher un nœud API.
	}
}

func RegisterView(actorID, postID int64) {
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

func flushInteractionsPeriodically() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop() // Bonne pratique pour libérer le timer
	ctx := context.Background()

	// On pré-alloue le batch pour soulager le Garbage Collector
	batch := make([]Interaction, 0, 5000)

	for {
		select {
		case interaction := <-interactionChan:
			batch = append(batch, interaction)

			// Si le batch devient très gros (ex: viralité), on vide immédiatement
			// sans attendre le tick de 5 secondes.
			if len(batch) >= 5000 {
				processCacheUpdates(ctx, batch)
				batch = make([]Interaction, 0, 5000)
			}

		case <-ticker.C:
			// Toutes les 5 secondes, on vide ce qu'on a, même s'il n'y en a que 2.
			if len(batch) > 0 {
				processCacheUpdates(ctx, batch)
				batch = make([]Interaction, 0, 5000)
			}
		}
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
	eventsToQueue = append(eventsToQueue, processMetricBatch(likesToAdd, "like")...)
	eventsToQueue = append(eventsToQueue, processMetricBatch(viewsToAdd, "view")...)

	// 3. Envoi vers le REQUEST Cache
	for _, event := range eventsToQueue {
		// On extrait proprement le target_id du payload (ex: ID du Post)
		payloadMap, ok := event.Payload.(map[string]interface{})
		if !ok {
			continue
		}

		targetID, ok := payloadMap["target_id"].(int64)
		if !ok {
			continue
		}

		// LE SECRET EST LÀ : partitionKey = targetID.
		// Le Like tombera dans le Shard de son Post parent.
		_ = redis.EnqueueDB(
			ctx,
			event.ID, // Auto-généré ou vide si c'est un compteur
			targetID, // partitionKey
			event.Type,
			event.Action,
			event.Payload,
			redis.TargetPostgres,
		)
	}
}

// processMetricBatch mutualise la logique de mise à jour pour les likes, vues, etc.
// Le contexte n'est plus nécessaire car cette fonction est désormais pure (sans I/O).
func processMetricBatch(items map[int64]int, metricType string) []redis.AsyncEvent {
	var events []redis.AsyncEvent

	for postID, count := range items {
		// LA SÉCURITÉ ARCHITECTURALE : Plus aucune modification de l'OBJECT Cache ici.
		// On délègue la responsabilité absolue de la mise à jour (L1 + ZSET) au Worker
		// pour éviter les "Race Conditions" (Read-Modify-Write) entre les instances de l'API.

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
				"target_id": postID,
				"count":     count,
				// On flag toujours à false, forçant le Worker à appliquer la valeur exacte BDD
				"already_evaluated_redis": false,
			},
		})
	}

	return events
}
