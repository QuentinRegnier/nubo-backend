// internal/cache/strategy_redis.go
package redis

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// ---------------- Sentinel (Gardien Mémoire) ----------------

// StartMemorySentinel surveille la RAM et déclenche l'éviction en gardant une marge de sécurité.
// securityMargin : Espace libre à garantir (ex: 200MB).
// Si Redis a 1GB de RAM et marge=200MB, on nettoie dès qu'on dépasse 800MB.
func StartMemorySentinel(ctx context.Context, rdb *redis.Client, securityMargin int64, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		log.Printf("[Sentinel] Démarrage surveillance dynamique (Marge de sécu: %d MB)", securityMargin/1024/1024)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkAndEvictDynamic(ctx, rdb, securityMargin)
			}
		}
	}()
}

// checkAndEvictDynamic calcule la limite en temps réel
func checkAndEvictDynamic(ctx context.Context, rdb *redis.Client, margin int64) {
	// 1. Récupérer toutes les infos mémoire d'un coup
	info, err := rdb.Info(ctx, "memory").Result()
	if err != nil {
		log.Printf("[Sentinel] Erreur lecture info mémoire: %v", err)
		return
	}

	// 2. Parser les valeurs clés
	usedMemory := parseRedisInfoInt(info, "used_memory")
	maxMemory := parseRedisInfoInt(info, "maxmemory")                   // Limite config Redis (redis.conf)
	totalSystemMemory := parseRedisInfoInt(info, "total_system_memory") // RAM totale du serveur/conteneur

	// 3. Déterminer la limite réelle (Plafond)
	var ceiling int64
	if maxMemory > 0 {
		ceiling = maxMemory
	} else {
		ceiling = totalSystemMemory
	}

	// Si on n'arrive pas à lire la mémoire système (cas rare), on ne fait rien par sécurité
	if ceiling == 0 {
		return
	}

	// 4. Calculer le seuil de déclenchement (Plafond - Marge)
	triggerLimit := ceiling - margin

	// Si on est en dessous du seuil, tout va bien
	if usedMemory < triggerLimit {
		return
	}

	log.Printf("[Sentinel] ⚠️ ALERTE RAM: Utilisé %d MB > Seuil %d MB (Plafond %d - Marge %d). Nettoyage...",
		usedMemory/1024/1024, triggerLimit/1024/1024, ceiling/1024/1024, margin/1024/1024)

	// 5. Boucle de suppression (inchangée par rapport à avant)
	evictedCount := 0
	safetyLoop := 0

	// On boucle tant qu'on dépasse la limite dynamique
	for usedMemory > triggerLimit && safetyLoop < 50 {
		items, err := rdb.ZPopMin(ctx, "idx:lru:global", 50).Result()
		if err != nil || len(items) == 0 {
			break
		}

		for _, item := range items {
			member, ok := item.Member.(string)
			if !ok {
				continue
			}

			parts := strings.SplitN(member, ":", 2)
			if len(parts) != 2 {
				continue
			}
			collName, id := parts[0], parts[1]

			registryMu.RLock()
			coll, exists := collectionRegistry[collName]
			registryMu.RUnlock()

			if exists {
				_ = coll.Delete(ctx, map[string]any{"id": id})
				evictedCount++
			}
		}

		// Mise à jour rapide de la mémoire utilisée pour la boucle
		// On refait un petit appel léger juste pour 'used_memory'
		usedMemory, _ = getRedisMemoryUsage(ctx, rdb)
		safetyLoop++
	}

	if evictedCount > 0 {
		log.Printf("[Sentinel] Fin cycle: %d supprimés. RAM actuelle: %d MB", evictedCount, usedMemory/1024/1024)
	}
}

// Helper générique pour parser les champs de INFO MEMORY
func parseRedisInfoInt(info string, key string) int64 {
	lines := strings.Split(info, "\r\n")
	prefix := key + ":"
	for _, line := range lines {
		if after, ok := strings.CutPrefix(line, prefix); ok {
			valStr := after
			val, _ := strconv.ParseInt(valStr, 10, 64)
			return val
		}
	}
	return 0
}

// getRedisMemoryUsage récupère la mémoire utilisée actuelle via INFO MEMORY
// Utilisé principalement dans la boucle de nettoyage pour vérifier si on est repassé sous le seuil.
func getRedisMemoryUsage(ctx context.Context, rdb *redis.Client) (int64, error) {
	info, err := rdb.Info(ctx, "memory").Result()
	if err != nil {
		return 0, err
	}
	// Réutilisation du helper pour éviter la duplication de code
	return parseRedisInfoInt(info, "used_memory"), nil
}

// ---------------- Flux ----------------

// DefaultFluxTTL est le temps de vie par défaut d'un message de flux
const DefaultFluxTTL = 1 * time.Second

// PushFluxWithTTL publie un message sur un flux et crée un TTL individuel
func PushFluxWithTTL(rdb *redis.Client, nodeName string, messageID string, message []byte, ttl time.Duration) error {
	ctx := context.Background()

	// Stocke le message temporairement avec TTL individuel
	key := "fluxmsg:" + messageID
	if err := rdb.Set(ctx, key, message, ttl).Err(); err != nil {
		return err
	}

	// Publie sur le canal pour diffusion immédiate
	channel := "flux:" + nodeName
	if err := rdb.Publish(ctx, channel, messageID).Err(); err != nil {
		return err
	}

	return nil
}

// SubscribeFlux s'abonne à un flux et renvoie les messages via un channel Go
func SubscribeFlux(rdb *redis.Client, nodeName string) (<-chan []byte, context.CancelFunc) {
	channel := "flux:" + nodeName
	ctx, cancel := context.WithCancel(context.Background())

	pubsub := rdb.Subscribe(ctx, channel)
	ch := make(chan []byte, 100) // buffer côté Go

	go func() {
		defer pubsub.Close()
		for msg := range pubsub.Channel() {
			messageID := msg.Payload
			// Récupère le message stocké temporairement
			data, err := rdb.Get(ctx, "fluxmsg:"+messageID).Bytes()
			if err == redis.Nil {
				continue // TTL déjà expiré
			} else if err != nil {
				log.Println("Erreur récupération flux message:", err)
				continue
			}
			ch <- data
		}
		close(ch)
	}()

	return ch, cancel
}
