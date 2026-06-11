package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
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
	UsersLite      *Collection
	ConvMeta       *Collection
	ConvMembers    *Collection
	SpeedFollowers *Collection
	SpeedRelations *Collection

	// --- FEED cache Collections ---
	FeedsObject       *Collection
	FeedsMailbox      *Collection
	FeedsPersonalized *Collection

	// --- ALGORITHM Cache Collections ---
	ContentVectors *Collection

	// --- SYSTEM Collections ---
	RateLimits   *Collection
	DLQ          *Collection
	GraphEdges   *Collection
	Tags         *Collection
	HashtagCanon *Collection

	// --- INDEX & IDEMPOTENCE ---
	SessionIndexes  *Collection
	PostLikesSet    *Collection
	CommentLikesSet *Collection
	SystemStatus    *Collection

	// --- Filtre Cuckoo distribué ---
	CuckooSeen       *Collection
	TrendGlobalDaily *Collection
	TrendTagDaily    *Collection

	// --- MESSAGING & LSH ---
	ConvParticipants *Collection
	UserInbox        *Collection
	LSHBuckets       *Collection

	// --- Activity Feed ---
	FeedSchedule *Collection
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
	SpeedFollowers = NewCollection("speed:followers", variables.StandardTTL)
	SpeedRelations = NewCollection("speed:relations", variables.StandardTTL)

	// --- FEED Cache ---
	FeedsObject = NewCollection("feed:state", variables.StandardTTL)
	FeedsMailbox = NewCollection("feed_cache:mailbox", variables.StandardTTL)
	FeedsPersonalized = NewCollection("feed:personalized", variables.StandardTTL)

	// --- ALGORITHM Cache (TDD §4.1 : LFU Object Store, TTL = 7 jours) ---
	ContentVectors = NewCollection("most_cache:vec", variables.StandardTTL)

	// --- SYSTEM Cache ---
	RateLimits = NewCollection("rate_limit:ip", 10*time.Second)
	DLQ = NewCollection("dlq", 0)                          // TTL infini pour la Dead Letter Queue
	GraphEdges = NewCollection("graph_cache:tag_edges", 0) // Remplace le formatage de clé manuel
	Tags = NewCollection("tags", 0)
	HashtagCanon = NewCollection("hashtag:canon", 0)

	// --- INDEX & IDEMPOTENCE ---
	SessionIndexes = NewCollection("session_cache", variables.StandardTTL)
	PostLikesSet = NewCollection("post:likes_set", 0)
	CommentLikesSet = NewCollection("comment:likes_set", 0)
	SystemStatus = NewCollection("system:status", 0)

	// --- Filtre Cuckoo distribué ---
	CuckooSeen = NewCollection("cuckoo:seen", variables.StandardTTL)
	TrendGlobalDaily = NewCollection("trend:global:daily", 0)
	TrendTagDaily = NewCollection("trend:tag", 0)

	// --- MESSAGING & LSH ---
	ConvParticipants = NewCollection("conv:participants", 0)
	UserInbox = NewCollection("inbox:user", 0)
	LSHBuckets = NewCollection("lsh:bucket", variables.StandardTTL)

	// --- Activity Feed ---
	FeedSchedule = NewCollection("feed:precompute:schedule", 0)
}

// IsReady isole l'état de l'infrastructure pour les routines de maintenance administratives.
func IsReady() bool {
	return redisgo.Rdb != nil
}

// CFExists encapsule la vérification probabiliste native de RedisBloom (O(1)).
func (c *Collection) CFExists(ctx context.Context, id any, item any) (bool, error) {
	return c.Client.Do(ctx, "CF.EXISTS", c.Key(id), item).Bool()
}

// CFAdd délègue l'allocation ou l'écriture atomique dans le filtre Cuckoo de l'écosystème Redis.
func (c *Collection) CFAdd(ctx context.Context, id any, item any) error {
	return c.Client.Do(ctx, "CF.ADD", c.Key(id), item).Err()
}

// Keys renvoie toutes les clés correspondant à un pattern global (Administration).
func Keys(ctx context.Context, pattern string) ([]string, error) {
	return redisgo.Rdb.Keys(ctx, pattern).Result()
}

// FlushDB vide l'intégralité de la base de données Redis courante de manière synchrone.
func FlushDB(ctx context.Context) error {
	return redisgo.Rdb.FlushDB(ctx).Err()
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

// --- Primitives ZSET de Collection ---

func (c *Collection) ZAdd(ctx context.Context, id any, score float64, member any) error {
	return c.Client.ZAdd(ctx, c.Key(id), &redis.Z{Score: score, Member: member}).Err()
}

// ZRem supprime des membres d'un ZSET.
func (c *Collection) ZRem(ctx context.Context, id any, members ...any) error {
	return c.Client.ZRem(ctx, c.Key(id), members...).Err()
}

// ZRangeByScoreWithLimit extrait les membres dont le score est inférieur ou égal à un seuil maximum (Batching O(log(N) + M)).
func (c *Collection) ZRangeByScoreWithLimit(ctx context.Context, id any, maxScore int64, limit int64) ([]string, error) {
	return c.Client.ZRangeByScore(ctx, c.Key(id), &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(maxScore, 10),
		Offset: 0,
		Count:  limit,
	}).Result()
}

// ---------------- CRUD OBJET (Single) ----------------

// SetObject stocke une struct Go en MsgPack dans Redis avec le TTL par défaut.
func (c *Collection) SetObject(ctx context.Context, id any, data any) error {
	// 1. Sérialisation MsgPack (Binaire, Rapide & Ultra léger en RAM)
	msgpackBytes, err := msgpack.Marshal(data)
	if err != nil {
		return fmt.Errorf("redis marshal nubo_error: %w", err)
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

// Incr incrémente une valeur numérique et retourne le nouveau compteur (ex: Rate Limiting).
func (c *Collection) Incr(ctx context.Context, id any) (int64, error) {
	return c.Client.Incr(ctx, c.Key(id)).Result()
}

// LPush ajoute un élément en tête de liste (ex: DLQ).
func (c *Collection) LPush(ctx context.Context, id any, values ...any) error {
	return c.Client.LPush(ctx, c.Key(id), values...).Err()
}

// Pipeline retourne un Pipeliner Redis depuis le client de la collection.
func (c *Collection) Pipeline() redis.Pipeliner {
	return c.Client.Pipeline()
}

// --- Primitives HASH ---

func (c *Collection) HGet(ctx context.Context, id any, field string) *redis.StringCmd {
	return c.Client.HGet(ctx, c.Key(id), field)
}

func (c *Collection) HSet(ctx context.Context, id any, values ...any) error {
	return c.Client.HSet(ctx, c.Key(id), values...).Err()
}

func (c *Collection) HGetAll(ctx context.Context, id any) *redis.StringStringMapCmd {
	return c.Client.HGetAll(ctx, c.Key(id))
}

func (c *Collection) HDel(ctx context.Context, id any, fields ...string) error {
	return c.Client.HDel(ctx, c.Key(id), fields...).Err()
}

// --- Primitives SET ---

func (c *Collection) SAdd(ctx context.Context, id any, members ...any) error {
	return c.Client.SAdd(ctx, c.Key(id), members...).Err()
}

func (c *Collection) SRem(ctx context.Context, id any, members ...any) error {
	return c.Client.SRem(ctx, c.Key(id), members...).Err()
}

func (c *Collection) SMembers(ctx context.Context, id any) ([]string, error) {
	return c.Client.SMembers(ctx, c.Key(id)).Result()
}

func (c *Collection) SCard(ctx context.Context, id any) (int64, error) {
	return c.Client.SCard(ctx, c.Key(id)).Result()
}

// SetPrimitive stocke une valeur brute sans MsgPack (utile pour les index pointant vers des IDs)
func (c *Collection) SetPrimitive(ctx context.Context, id any, val any) error {
	return c.Client.Set(ctx, c.Key(id), val, c.DefaultTTL).Err()
}

// GetInt64 récupère une valeur primitive brute sous forme d'entier
func (c *Collection) GetInt64(ctx context.Context, id any) (int64, error) {
	return c.Client.Get(ctx, c.Key(id)).Int64()
}

// MGet expose l'accès multiple brut. Préférer GetMany quand c'est possible, mais vital pour les clés composites.
func (c *Collection) MGet(ctx context.Context, ids ...any) ([]interface{}, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = c.Key(id)
	}
	return c.Client.MGet(ctx, keys...).Result()
}

// SAddCount ajoute au SET et retourne le nombre d'éléments réellement ajoutés (indispensable pour l'idempotence)
func (c *Collection) SAddCount(ctx context.Context, id any, members ...any) (int64, error) {
	return c.Client.SAdd(ctx, c.Key(id), members...).Result()
}

// SRemCount supprime du SET et retourne le nombre d'éléments réellement supprimés
func (c *Collection) SRemCount(ctx context.Context, id any, members ...any) (int64, error) {
	return c.Client.SRem(ctx, c.Key(id), members...).Result()
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
