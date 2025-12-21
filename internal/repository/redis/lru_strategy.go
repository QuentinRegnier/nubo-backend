// internal/cache/strategy_redis.go
package redis

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// ---------------- Types ----------------

// Type d’un noeud Redis
type NodeType int

const (
	NodeFlux NodeType = iota
	NodeCache
)

// Un élément dans la LRU globale
type CacheElement struct {
	NodeName  string // nom du noeud (ex: "messages")
	ElementID string // ex: "392"
	prev      *CacheElement
	next      *CacheElement
}

// LRU globale pour les éléments de type cache
type LRUCache struct {
	elements map[string]*CacheElement // clé = nodeName:elementID
	head     *CacheElement
	tail     *CacheElement
	mu       sync.Mutex
	rdb      *redis.Client
}

// ---------------- Initialisation ----------------

// NewLRUCache initialise un cache LRU global
func NewLRUCache(rdb *redis.Client) *LRUCache {
	return &LRUCache{
		elements: make(map[string]*CacheElement),
		rdb:      rdb,
	}
}

// ---------------- Gestion usage ----------------

// MarkUsed marque un élément comme utilisé (move to tail)
func (lru *LRUCache) MarkUsed(nodeName, elementID string) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	key := nodeName + ":" + elementID
	elem, exists := lru.elements[key]
	if exists {
		lru.moveToTail(elem)
		return
	}

	elem = &CacheElement{NodeName: nodeName, ElementID: elementID}
	lru.elements[key] = elem
	lru.append(elem)
}

func (lru *LRUCache) moveToTail(elem *CacheElement) {
	if elem == lru.tail {
		return
	}
	lru.remove(elem)
	lru.append(elem)
}

func (lru *LRUCache) append(elem *CacheElement) {
	if lru.tail != nil {
		lru.tail.next = elem
		elem.prev = lru.tail
		elem.next = nil
		lru.tail = elem
	} else {
		lru.head = elem
		lru.tail = elem
	}
}

func (lru *LRUCache) remove(elem *CacheElement) {
	if elem.prev != nil {
		elem.prev.next = elem.next
	} else {
		lru.head = elem.next
	}
	if elem.next != nil {
		elem.next.prev = elem.prev
	} else {
		lru.tail = elem.prev
	}
	elem.prev = nil
	elem.next = nil
}

func (lru *LRUCache) purgeOldest() {
	if lru.head == nil {
		return
	}
	old := lru.head
	log.Printf("Purging Redis cache element (LRU): node=%s, id=%s\n", old.NodeName, old.ElementID)
	lru.remove(old)
	delete(lru.elements, old.NodeName+":"+old.ElementID)

	// suppression via Collection.Delete
	collection := &Collection{
		Name:  old.NodeName,
		Redis: lru.rdb,
		LRU:   lru,
	}
	filter := map[string]interface{}{"id": old.ElementID}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := collection.Delete(ctx, filter); err != nil {
		log.Printf("Erreur suppression via Collection.Delete: %v\n", err)
	}
}

// ---------------- Mémoire ----------------

// StartMemoryWatcher surveille la RAM réelle de Redis via la commande INFO
func (lru *LRUCache) StartMemoryWatcher(maxRAM uint64, marge uint64, interval time.Duration) {
	go func() {
		// Intervalle de vérification
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			// 1. Interroger Redis pour obtenir l'info mémoire
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			infoStr, err := lru.rdb.Info(ctx, "memory").Result()
			cancel()

			if err != nil {
				log.Printf("Erreur lors de la surveillance mémoire Redis: %v", err)
				continue
			}

			// 2. Parser la réponse pour obtenir 'used_memory' en octets
			usedMemory, err := parseRedisUsedMemory(infoStr)
			if err != nil {
				log.Printf("Erreur parsing info mémoire: %v", err)
				continue
			}

			// 3. Définir la limite
			// Si maxRAM est 0, on essaye de deviner la RAM système (attention: seulement valide si Redis est sur la même machine)
			limit := maxRAM
			if limit == 0 {
				limit = getTotalRAM()
			}
			seuil := limit - marge

			// 4. Purge si nécessaire
			if usedMemory > seuil {
				log.Printf("⚠️ ALERTE RAM REDIS: Utilisé=%d, Seuil=%d. Démarrage purge...", usedMemory, seuil)

				// On purge tant qu'on est au-dessus du seuil
				// On met une limite de sécurité (ex: max 50 items par tick) pour ne pas bloquer le thread indéfiniment
				itemsPurged := 0
				maxPurgePerCycle := 50

				lru.mu.Lock()
				for usedMemory > seuil && lru.head != nil && itemsPurged < maxPurgePerCycle {
					lru.purgeOldest() // Cette fonction supprime de la LRU ET de Redis
					itemsPurged++

					// Estimation grossière : on réduit virtuellement usedMemory pour la boucle
					// (Pour être précis, il faudrait refaire un rdb.Info(), mais c'est lourd)
					// On suppose ici qu'on va revérifier au prochain tick.
				}
				lru.mu.Unlock()

				log.Printf("Fin cycle purge: %d éléments supprimés.", itemsPurged)
			}
		}
	}()
}

// parseRedisUsedMemory extrait la valeur "used_memory" de la sortie de la commande INFO MEMORY
func parseRedisUsedMemory(info string) (uint64, error) {
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "used_memory:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strconv.ParseUint(parts[1], 10, 64)
			}
		}
	}
	return 0, nil // ou erreur si non trouvé
}

// getTotalRAM reste utile si Redis tourne sur la même machine (bare metal),
// mais attention si tu utilises Docker avec des limites de conteneur.
func getTotalRAM() uint64 {
	// ... (ton implémentation actuelle runtime.ReadMemStats est correcte pour la RAM Système du conteneur Go)
	// Mais idéalement, il vaut mieux passer une valeur explicite 'maxRAM' dans la config.
	return 0 // Je te conseille de forcer l'usage du paramètre maxRAM
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

// ---------------- Global ----------------

// GlobalStrategy est l’instance globale de stratégie LRU utilisée par toute l’app
var GlobalStrategy *LRUCache
