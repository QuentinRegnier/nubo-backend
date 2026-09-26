package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// Interaction représente un micro-événement de la messagerie (Réaction ou Non-lu).
type Interaction struct {
	ActorID   int64
	TargetID  int64
	Type      string
	Timestamp int64
	Emoji     string // Spécifique pour les réactions ("❤️", "😂")
	Delta     int    // Direction du compteur (+1 ou -1)
}

// Canal asynchrone bufferisé global pour absorber les pics de charge (Backpressure)
var interactionChan = make(chan Interaction, variables.InteractionBufferSize)

// init lance automatiquement la goroutine de vidage (Flusher) au démarrage du package.
func init() {
	go flushInteractionsPeriodically()
}

// ############################################################################
// # WORKER : INTERACTION BATCHER (TAMPON MÉMOIRE ANTI-DDOS POUR LA BDD)
// ############################################################################

// RegisterUnread place dans la file d'attente une demande d'incrémentation
// de message non lu pour un utilisateur spécifique dans une conversation.
func RegisterUnread(convID int64, userID int64) {
	select {
	case interactionChan <- Interaction{
		ActorID:   userID, // Le destinataire qui reçoit l'Unread
		TargetID:  convID, // La conversation concernée
		Type:      "unread",
		Timestamp: time.Now().Unix(),
	}:
	default:
		// SÉCURITÉ (Backpressure) : Si le tampon est plein, on perd le compteur
		// silencieusement plutôt que de crasher le thread HTTP. Le Fast Path L1
		// et la fonction de guèrison automatique de func_get_member compenseront.
		logger.Log.Warn().Int64("user_id", userID).Msg("Interaction Worker : Tampon RAM plein, perte d'un compteur Non-Lu")
	}
}

// RegisterMessageReaction place dans la file d'attente une demande de modification
// de compteur de réaction sur un message.
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
		logger.Log.Warn().Int64("msg_id", msgID).Msg("Interaction Worker : Tampon RAM plein, perte d'un delta de réaction")
	}
}

// flushInteractionsPeriodically est la boucle infinie qui consomme le tampon RAM
// et envoie des paquets agrégés vers la file Redis.
func flushInteractionsPeriodically() {
	ticker := time.NewTicker(variables.InteractionFlushInterval)
	defer ticker.Stop()

	ctx := context.Background()
	batch := make([]Interaction, 0, variables.InteractionBatchThreshold)

	for {
		select {
		case interaction := <-interactionChan:
			batch = append(batch, interaction)

			// Vidage immédiat en cas de viralité extrême (Atteinte du Threshold)
			if len(batch) >= variables.InteractionBatchThreshold {
				processCacheUpdates(ctx, batch)
				batch = make([]Interaction, 0, variables.InteractionBatchThreshold)
			}

		case <-ticker.C:
			// Vidage chronologique régulier (Toutes les X secondes)
			if len(batch) > 0 {
				processCacheUpdates(ctx, batch)
				batch = make([]Interaction, 0, variables.InteractionBatchThreshold)
			}
		}
	}
}

// processCacheUpdates agrège mathématiquement les paquets de compteurs en RAM
// avant de les formater et de les expédier vers les bases de données via le Write-Behind.
func processCacheUpdates(ctx context.Context, batch []Interaction) {

	// ── ÉTAPE 1 : AGRÉGATION MATHÉMATIQUE EN RAM (RÉDUCTION DES I/O) ────────
	unreadsToAdd := make(map[string]int)             // Clé composée: "convID:userID"
	reactionsToAdd := make(map[int64]map[string]int) // Map tridimensionnelle: msgID -> emoji -> delta

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

	// ── ÉTAPE 2 : FORMATAGE DES ÉVÉNEMENTS (PURIFICATION) ───────────────────
	for msgID, emojiDeltas := range reactionsToAdd {

		// Nettoyage: On isole les Deltas réels (Si un utilisateur a cliqué puis décliqué dans
		// la même fenêtre de temps de 5 secondes, le delta global est de 0, on l'annule en RAM).
		validDeltas := make(map[string]int)
		for emoji, delta := range emojiDeltas {
			if delta != 0 {
				validDeltas[emoji] = delta
			}
		}

		if len(validDeltas) > 0 {
			eventsToQueue = append(eventsToQueue, redis.AsyncEvent{
				Type:   redis.EntityMessage, // Cible = Mise à jour du JSONB du Message parent
				Action: redis.ActionBuild,   // Code action spécifique pour une modification partielle
				Payload: map[string]interface{}{
					"message_id": msgID,
					"deltas":     validDeltas,
				},
				Targets: redis.TargetPostgres | redis.TargetMongo, // Dispatch L2 et L3
			})
		}
	}

	// ── ÉTAPE 3 : ENVOI SUR LA FILE REDIS (WRITE-BEHIND) ────────────────────
	for _, event := range eventsToQueue {
		payloadMap, ok := event.Payload.(map[string]interface{})
		if !ok {
			continue
		}

		// Définition de la Clé de Partitionnement (Sharding Redis) pour assurer l'ordre.
		var partitionKey int64
		if event.Type == redis.EntityMembers {
			partitionKey = payloadMap["conversation_id"].(int64)
		} else if event.Type == redis.EntityMessage {
			partitionKey = payloadMap["message_id"].(int64)
		} else {
			partitionKey = payloadMap["target_id"].(int64)
		}

		// Transfert de la charge au Worker Principal (worker.go)
		errEnqueue := redis.EnqueueDB(
			ctx,
			event.ID,
			partitionKey,
			event.Type,
			event.Action,
			event.Payload,
			event.Targets,
		)

		if errEnqueue != nil {
			logger.Log.Error().
				Err(errEnqueue).
				Interface("event_type", event.Type).
				Msg("Interaction Worker : Échec critique de l'Enqueue vers les bases de données")
		}
	}
}
