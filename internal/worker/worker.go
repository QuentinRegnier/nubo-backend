package worker

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// --- CONFIGURATION DU CERVEAU (Modifiable via .env) ---
var (
	MaxBatchSize int64
	MinBackoff   time.Duration
	MaxBackoff   time.Duration
)

func init() {
	// 1. Taille du Batch (Ex: 5000)
	if val, err := strconv.ParseInt(os.Getenv("WORKER_MAX_BATCH_SIZE"), 10, 64); err == nil && val > 0 {
		MaxBatchSize = val
	} else {
		MaxBatchSize = 5000 // Valeur par défaut
	}

	// 2. Backoff Minimum (Période d'hyperactivité, ex: 50ms)
	if val, err := strconv.Atoi(os.Getenv("WORKER_MIN_BACKOFF_MS")); err == nil && val > 0 {
		MinBackoff = time.Duration(val) * time.Millisecond
	} else {
		MinBackoff = 50 * time.Millisecond // Valeur par défaut
	}

	// 3. Backoff Maximum (Sommeil profond, ex: 1000ms)
	if val, err := strconv.Atoi(os.Getenv("WORKER_MAX_BACKOFF_MS")); err == nil && val > 0 {
		MaxBackoff = time.Duration(val) * time.Millisecond
	} else {
		MaxBackoff = 1 * time.Second // Valeur par défaut
	}
}

func runWorker(ctx context.Context, shardID int) {
	currentBackoff := MinBackoff

	for {
		// 1. Vérification de l'arrêt gracieux du serveur
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 2. Blocage absolu (0 CPU) via BLMPOP / BLPOP
		// On limite la taille via MaxBatchSize (dynamique)
		events, err := redis.PopSmartBatchBlocking(ctx, shardID, MaxBatchSize)

		if err != nil {
			log.Printf("⚠️ Worker %d: Erreur Redis: %v", shardID, err)
			time.Sleep(1 * time.Second) // Protection anti-boucle infinie si Redis crashe
			continue
		}

		// 3. Traitement dynamique
		if len(events) > 0 {
			processBatch(ctx, events)
			// RESET DU SOMMEIL : on a trouvé du travail, on repasse à la vitesse max !
			currentBackoff = MinBackoff
		} else {
			// SLEEP : la file était vide (malgré le blocage), on s'endort doucement
			time.Sleep(currentBackoff)
			currentBackoff *= 2
			if currentBackoff > MaxBackoff {
				currentBackoff = MaxBackoff
			}
		}
	}
}

// processBatch trie les événements et les envoie aux bases ET au cache_service
func processBatch(ctx context.Context, events []redis.AsyncEvent) {

	// 🛡️ BOUCLIER DE SÉCURITÉ ASYNCHRONE
	// On purge les événements illégaux AVANT de les distribuer aux BDD et aux Caches.
	validEvents := purifyBatch(ctx, events)

	var mongoEvents []redis.AsyncEvent
	var pgEvents []redis.AsyncEvent
	var workerEvents []redis.AsyncEvent // <-- NOUVEAU

	for _, evt := range validEvents {
		if evt.Targets&redis.TargetMongo != 0 {
			mongoEvents = append(mongoEvents, evt)
		}
		if evt.Targets&redis.TargetPostgres != 0 {
			pgEvents = append(pgEvents, evt)
		}
		if evt.Targets&redis.TargetWorker != 0 { // <-- NOUVEAU
			workerEvents = append(workerEvents, evt)
		}
	}

	// Étape 1 : Exécution Parallèle des bases de données (Mongo & Postgres)
	done := make(chan bool)

	go func() {
		if len(mongoEvents) > 0 {
			flushMongo(ctx, mongoEvents)
		}
		done <- true
	}()

	go func() {
		if len(pgEvents) > 0 {
			flushPostgres(ctx, pgEvents)
		}
		done <- true
	}()

	// Lancement des tâches internes (hors BDD) en parallèle
	go func() {
		if len(workerEvents) > 0 {
			processInternalJobs(ctx, workerEvents)
		}
	}()

	// 🛑 BARRIÈRE DE SYNCHRONISATION
	// On DOIT attendre que la BDD ait validé les transactions sur le disque
	// avant de mettre à jour le cache, sinon on lira des valeurs périmées.
	<-done
	<-done

	// ✅ ÉTAPE 2 : MISE À JOUR DES CACHES (Object Cache L1 & Most Cache)
	// À cet instant, on est certain que le disque est à jour.
	if len(validEvents) > 0 {
		// On lance les deux métiers de cache en parallèle
		go updateCountersCache(ctx, validEvents) // Le Secrétaire (Object Cache)
		go updateMostCache(ctx, validEvents)     // Le Cerveau (ZSETs et Recommandations)
	}

	// Étape 3 : Fan-Out Social de masse (Distribution dans les boîtes aux lettres du Speed Cache)
	// S'exécute de manière ultra-rapide en RAM juste après la validation BDD
	handleSocialFanOut(ctx, validEvents)
}

// purifyBatch agit comme un pare-feu asynchrone.
// Il élimine les événements illégaux (ex: un Like sur un post privé) pour protéger
// Postgres, Mongo, l'Object Cache et le Most Cache en un seul point de contrôle.
func purifyBatch(ctx context.Context, events []redis.AsyncEvent) []redis.AsyncEvent {
	validEvents := make([]redis.AsyncEvent, 0, len(events))

	for _, e := range events {
		// On inspecte les interactions ET les Commentaires
		if e.Type == redis.EntityLike || e.Type == redis.EntityView || e.Type == redis.EntityComment {
			jsonBytes, err := json.Marshal(e.Payload)
			if err != nil {
				continue // Payload corrompu, on drop
			}

			var payload struct {
				PostID   int64 `json:"post_id"`
				TargetID int64 `json:"target_id"` // Historique
				UserID   int64 `json:"user_id"`
			}

			if err := json.Unmarshal(jsonBytes, &payload); err == nil {
				targetID := payload.TargetID
				if payload.PostID != 0 {
					targetID = payload.PostID
				}

				if targetID != 0 && payload.UserID != 0 {
					// VÉRIFICATION DES DROITS (Cascade L1 -> L2 -> L3)
					// (getPostWithFallback est déjà défini dans most_cache_worker.go et accessible ici)
					p, err := getPostWithFallback(ctx, targetID)

					// Règle 1 : Post supprimé ou introuvable
					if err != nil || p.Visibility == -1 {
						continue // Événement détruit
					}

					// Règle 2 : Matrice de Confidentialité
					isAuthor := p.UserID == payload.UserID
					if !isAuthor {
						relationState := cache_service.RelationValue(ctx, p.UserID, payload.UserID)
						if relationState == -1 {
							continue // Hacker banni -> Événement détruit
						}
						if p.Visibility == 1 && relationState < 1 {
							continue // Réservé Abonnés -> Événement détruit
						}
						if p.Visibility == 2 && relationState != 2 {
							continue // Réservé Amis -> Événement détruit
						}
					}
				}
			}
		}
		// Si l'événement survit au pare-feu (ou si ce n'est pas une interaction), on l'accepte
		validEvents = append(validEvents, e)
	}

	return validEvents
}
