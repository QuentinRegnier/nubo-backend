// internal/cache/strategy_redis.go
package cache

import (
	"context"
	"log"
	"runtime"
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

	// suppression dans Redis
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	key := "cache:" + old.NodeName
	if err := lru.rdb.HDel(ctx, key, old.ElementID).Err(); err != nil {
		log.Printf("Erreur suppression Redis: %v\n", err)
	}
}

// ---------------- Mémoire ----------------

// StartMemoryWatcher surveille la RAM
func (lru *LRUCache) StartMemoryWatcher(maxRAM uint64, marge uint64, interval time.Duration) {
	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			used := m.Alloc
			if maxRAM == 0 {
				maxRAM = getTotalRAM()
			}
			if used > maxRAM-marge {
				log.Printf("RAM utilisée=%d, dépassement seuil=%d, purge LRU...\n", used, maxRAM-marge)
				lru.mu.Lock()
				lru.purgeOldest()
				lru.mu.Unlock()
			}
			time.Sleep(interval)
		}
	}()
}

func getTotalRAM() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys
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

// ---------------- Cache ----------------

// SetCache ajoute un élément au cache
func (lru *LRUCache) SetCache(ctx context.Context, nodeName, elementID string, value []byte) error {
	key := "cache:" + nodeName
	if err := lru.rdb.HSet(ctx, key, elementID, value).Err(); err != nil {
		return err
	}
	lru.MarkUsed(nodeName, elementID)
	return nil
}

// GetCache lit un élément du cache
func (lru *LRUCache) GetCache(ctx context.Context, nodeName, elementID string) ([]byte, error) {
	key := "cache:" + nodeName
	val, err := lru.rdb.HGet(ctx, key, elementID).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err == nil {
		lru.MarkUsed(nodeName, elementID)
	}
	return val, err
}

// ---------------- Global ----------------

// GlobalStrategy est l’instance globale de stratégie LRU utilisée par toute l’app
var GlobalStrategy *LRUCache
