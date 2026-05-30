package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	variables "github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/go-redis/redis/v8"
)

// ---------------- Initialisation ----------------

var (
	// Users Collections d'Objets (JSON + TTL)
	Users        *Collection
	UserSettings *Collection
	Sessions     *Collection
	Posts        *Collection
	Comments     *Collection
	Likes        *Collection
	// Media Note : Likes peut être géré différemment (compteurs), mais on garde l'objet pour l'instant
	Media         *Collection
	Conversations *Collection
	Members       *Collection
	Messages      *Collection
	Relations     *Collection

	// --- SPEED Cache Collections ---
	UsersLite   *Collection
	ConvMeta    *Collection
	ConvMembers *Collection
)

func InitCacheDatabase() {
	// --- OBJECT Cache ---
	Users = NewCollection("object_cache:user", variables.StandardTTL)
	UserSettings = NewCollection("object_cache:user_settings", variables.StandardTTL)
	Sessions = NewCollection("object_cache:session", variables.StandardTTL)

	Posts = NewCollection("object_cache:post_service", variables.StandardTTL)
	Comments = NewCollection("object_cache:comment", variables.StandardTTL)
	Media = NewCollection("object_cache:media", variables.StandardTTL)
	Messages = NewCollection("object_cache:msg", variables.StandardTTL)

	Likes = NewCollection("object_cache:like", variables.StandardTTL)
	Conversations = NewCollection("object_cache:conv", variables.StandardTTL)
	Members = NewCollection("object_cache:member", variables.StandardTTL)
	Relations = NewCollection("object_cache:rel", variables.StandardTTL)

	// --- SPEED Cache (Lite Objects) ---
	UsersLite = NewCollection("speed_cache:user_lite", variables.StandardTTL)
	ConvMeta = NewCollection("speed_cache:conv_meta", variables.StandardTTL)
	ConvMembers = NewCollection("speed_cache:conv_members", variables.StandardTTL)
}

// ---------------- Collection ----------------

type Collection struct {
	Prefix     string        // ex: "post_service" (donnera "post_service:123")
	Client     *redis.Client // Client Redis
	DefaultTTL time.Duration // Durée de vie par défaut (ex : 7 jours)
}

func NewCollection(prefix string, ttl time.Duration) *Collection {
	return &Collection{
		Prefix:     prefix,
		Client:     redisgo.Rdb,
		DefaultTTL: ttl,
	}
}

// Key génère la clé Redis finale : "prefix:id"
func (c *Collection) Key(id any) string {
	return fmt.Sprintf("%s:%v", c.Prefix, id)
}

// ---------------- CRUD OBJET (Single) ----------------

// SetObject stocke une struct Go en MsgPack dans Redis avec le TTL par défaut.
func (c *Collection) SetObject(ctx context.Context, id any, data any) error {
	// 1. Sérialisation MsgPack (Binaire, Rapide & Ultra léger en RAM)
	msgpackBytes, err := msgpack.Marshal(data)
	if err != nil {
		return fmt.Errorf("redis marshal error: %w", err)
	}

	// 2. SET avec Expiration (Indispensable pour volatile lfu)
	key := c.Key(id)
	return c.Client.Set(ctx, key, msgpackBytes, c.DefaultTTL).Err()
}

// GetObject récupère un objet et le désérialise dans 'dest'.
// Retourne redis.Nil si non trouvé.
func (c *Collection) GetObject(ctx context.Context, id any, dest any) error {
	key := c.Key(id)

	// 1. GET Raw Bytes
	val, err := c.Client.Get(ctx, key).Bytes()
	if err != nil {
		return err // redis.Nil si absent
	}

	// 2. Désérialisation MsgPack
	return msgpack.Unmarshal(val, dest)
}

// DeleteObject supprime un objet du cache_service.
func (c *Collection) DeleteObject(ctx context.Context, id any) error {
	return c.Client.Del(ctx, c.Key(id)).Err()
}

// RefreshTTL prolonge la durée de vie d'un objet (utile pour les sessions actives).
func (c *Collection) RefreshTTL(ctx context.Context, id any) error {
	return c.Client.Expire(ctx, c.Key(id), c.DefaultTTL).Err()
}

// ---------------- PIPELINE D'HYDRATATION (Massive Read) ----------------

// GetManyResult contient le résultat d'un chargement en masse
type GetManyResult struct {
	Found      map[int64][]byte // Map ID -> MsgPack Raw
	MissingIDs []int64          // Liste des IDs non trouvée (à demander à Mongo)
}

// GetMany récupère une liste d'objets en un seul appel réseau (MGET).
// C'est le cœur de la performance pour les Feeds.
func (c *Collection) GetMany(ctx context.Context, ids []int64) (*GetManyResult, error) {
	if len(ids) == 0 {
		return &GetManyResult{}, nil
	}

	// 1. Préparer les clés
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = c.Key(id)
	}

	// 2. MGET (Multi-Get) : 1 seul Round-Trip vers Redis
	// Valable pour des string keys (notre cas JSON)
	values, err := c.Client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	// 3. Analyser les résultats (Hit vs Miss)
	result := &GetManyResult{
		Found:      make(map[int64][]byte),
		MissingIDs: make([]int64, 0),
	}

	for i, val := range values {
		originalID := ids[i]

		if val == nil {
			// MISS : Le LFU a supprimé cet objet, ou il n'a jamais été là.
			result.MissingIDs = append(result.MissingIDs, originalID)
		} else {
			// HIT : On a la donnée (string/json)
			if strVal, ok := val.(string); ok {
				result.Found[originalID] = []byte(strVal)
			} else {
				// Cas rare (corruption ?), on considère comme manquant
				result.MissingIDs = append(result.MissingIDs, originalID)
			}
		}
	}

	return result, nil
}

// ---------------- Flux (Pattern Claim Check) ----------------

// DefaultFluxTTL est le temps de vie par défaut d'un message de flux (court car transitoire)
const DefaultFluxTTL = 5 * time.Second

// PushFluxWithTTL publie un ID sur un canal et stocke le contenu avec un TTL.
// C'est utile pour envoyer des données qui ne doivent pas persister longtemps mais doivent être lues rapidement.
func PushFluxWithTTL(rdb *redis.Client, nodeName string, messageID string, message []byte, ttl time.Duration) error {
	ctx := context.Background()

	// 1. Stocke le message temporairement (Claim Check)
	// La clé expire toute seule grâce à la config volatile-lfu ou au TTL natif
	key := "fluxmsg:" + messageID
	if err := rdb.Set(ctx, key, message, ttl).Err(); err != nil {
		return err
	}

	// 2. Publie l'ID sur le canal pour réveiller les abonnés
	channel := "flux:" + nodeName
	if err := rdb.Publish(ctx, channel, messageID).Err(); err != nil {
		return err
	}

	return nil
}

// SubscribeFlux s'abonne à un flux et récupère automatiquement le contenu des messages.
func SubscribeFlux(rdb *redis.Client, nodeName string) (<-chan []byte, context.CancelFunc) {
	channel := "flux:" + nodeName
	ctx, cancel := context.WithCancel(context.Background())

	// Abonnement Redis
	pubsub := rdb.Subscribe(ctx, channel)

	// Canal Go pour renvoyer les données décodées à l'application
	ch := make(chan []byte, 100)

	go func() {
		defer func(pubsub *redis.PubSub) {
			err := pubsub.Close()
			if err != nil {
			}
		}(pubsub)
		defer close(ch)

		// Boucle d'écoute
		for msg := range pubsub.Channel() {
			messageID := msg.Payload

			// 3. Récupération du contenu (Claim)
			// On utilise un court contexte pour cette lecture
			readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
			data, err := rdb.Get(readCtx, "fluxmsg:"+messageID).Bytes()
			readCancel()

			if errors.Is(redis.Nil, err) {
				// Le message a expiré ou a été supprimé avant qu'on le lise (Tant pis)
				continue
			} else if err != nil {
				log.Printf("⚠️ Erreur flux %s : impossible de lire le message %s : %v", nodeName, messageID, err)
				continue
			}

			// Envoi dans le canal Go
			select {
			case ch <- data:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, cancel
}
