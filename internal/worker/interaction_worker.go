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
	Emoji     string // NOUVEAU: Spécifique pour les réactions
	Delta     int    // NOUVEAU: +1 ou -1
}

var (
	// Canal asynchrone bufferisé (50 000 emplacements).
	interactionChan = make(chan Interaction, 50000)
)

func init() {
	go flushInteractionsPeriodically()
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

// RegisterMessageReaction met en file d'attente une mise à jour de compteur de réaction
func RegisterMessageReaction(msgID int64, emoji string, delta int) {
	select {
	case interactionChan <- Interaction{
		TargetID:  msgID,
		Type:      "msg_reaction",
		Emoji:     emoji,
		Delta:     delta,
		Timestamp: time.Now().Unix(),
	}:
	default:
		// BACKPRESSURE: Si le buffer RAM est plein, on perd le compteur (le Fast Path L1 est prioritaire)
		logger.Log.Warn().Int64("msg_id", msgID).Msg("Interaction Worker : Buffer plein, perte d'un delta de réaction")
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
	unreadsToAdd := make(map[string]int)             // Clé = "convID:userID"
	reactionsToAdd := make(map[int64]map[string]int) // NOUVEAU: map[msgID]map[emoji]delta

	// 1. Agrégation mathématique en RAM
	for _, interaction := range batch {
		if interaction.Type == "unread" {
			key := fmt.Sprintf("%d:%d", interaction.TargetID, interaction.ActorID)
			unreadsToAdd[key]++
		} else if interaction.Type == "msg_reaction" {
			if reactionsToAdd[interaction.TargetID] == nil {
				reactionsToAdd[interaction.TargetID] = make(map[string]int)
			}
			reactionsToAdd[interaction.TargetID][interaction.Emoji] += interaction.Delta
		}
	}

	var eventsToQueue []redis.AsyncEvent

	// 2. Traitement des Réactions aux messages (NOUVEAU)
	for msgID, emojiDeltas := range reactionsToAdd {
		// Nettoyage: on ne garde que les emojis avec un delta != 0
		validDeltas := make(map[string]int)
		for emoji, delta := range emojiDeltas {
			if delta != 0 {
				validDeltas[emoji] = delta
			}
		}

		if len(validDeltas) > 0 {
			eventsToQueue = append(eventsToQueue, redis.AsyncEvent{
				Type:   redis.EntityMessage, // On cible la mise à jour du Message parent
				Action: redis.ActionBuild,   // Action personnalisée pour signifier une mise à jour partielle
				Payload: map[string]interface{}{
					"message_id": msgID,
					"deltas":     validDeltas,
				},
				Targets: redis.TargetPostgres | redis.TargetMongo,
			})
		}
	}

	// 3. Envoi sur la File Redis (Write-Behind)
	for _, event := range eventsToQueue {
		payloadMap, ok := event.Payload.(map[string]interface{})
		if !ok {
			continue
		}

		var partitionKey int64
		if event.Type == redis.EntityMembers {
			partitionKey = payloadMap["conversation_id"].(int64)
		} else if event.Type == redis.EntityMessage {
			partitionKey = payloadMap["message_id"].(int64) // Routage par message_id
		} else {
			partitionKey = payloadMap["target_id"].(int64)
		}

		// On utilise TargetWorker pour les compteurs (les bases géreront ça spécifiquement)
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
