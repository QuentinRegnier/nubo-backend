package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	variables "github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/go-redis/redis/v8"
	"github.com/vmihailenco/msgpack/v5"
)

// ============================================================================
// 1. VARIABLES GLOBALES (LES COLLECTIONS DE L'APPLICATION)
// ============================================================================

var (
	// --- OBJECT Cache (JSON/MsgPack + TTL) ---
	Users            *Collection
	UserSettings     *Collection
	Sessions         *Collection
	Posts            *Collection
	Comments         *Collection
	Media            *Collection
	Conversations    *Collection
	Members          *Collection
	Messages         *Collection
	Relations        *Collection
	Saved            *Collection // ZSET des favoris de l'utilisateur
	PostComments     *Collection // ZSET des commentaires d'un post
	UserTimeline     *Collection // ZSET de la timeline d'un utilisateur
	NotificationsObj *Collection

	// --- SPEED Cache (Lite Objects & UI) ---
	UsersLite      *Collection
	UsersLex       *Collection // ZSET Lexicographique pour l'auto-complétion
	ConvMeta       *Collection
	ConvMembers    *Collection
	SpeedFollowers *Collection
	SpeedRelations *Collection
	SpeedAddable   *Collection

	// --- FEED Cache ---
	FeedsObject       *Collection
	FeedsMailbox      *Collection
	FeedsPersonalized *Collection

	// --- ALGORITHM & MOST Cache ---
	ContentVectors     *Collection
	RankedPosts        *Collection // Les ZSETs stricts (likes, views, recent)
	TagPosts           *Collection // Les ZSETs par tags pour l'UI
	TrendGlobalHourly  *Collection
	TrendGlobalDaily   *Collection
	TrendTagDaily      *Collection
	TrendTagWeekly     *Collection
	HashtagLeaderboard *Collection

	// --- SYSTEM Cache ---
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

	// --- CUCKOO FILTER ---
	CuckooSeen *Collection

	// --- MESSAGING & LSH ---
	ConvParticipants *Collection
	UserInbox        *Collection
	MessagesIndex    *Collection
	LSHBuckets       *Collection

	// --- ACTIVITY FEED ---
	FeedSchedule        *Collection
	NotificationsZSet   *Collection
	NotificationCursors *Collection
	InboxActivity       *Collection

	// --- TELEMETRY Cache ---
	TelemetryVectors    *Collection
	TelemetryTags       *Collection
	TelemetryTimestamps *Collection

	// --- WEBSOCKET Cache ---
	Presence *Collection

	// --- PUB/SUB CHANNELS & WORKER QUEUES ---
	ChannelUser      *Collection
	ChannelCommunity *Collection
	WorkerQueue      *Collection
)

// ============================================================================
// 2. INITIALISATION
// ============================================================================

func InitCacheDatabase() {
	// --- OBJECT Cache ---
	Users = NewCollection("object_cache:user", variables.StandardTTL)
	UserSettings = NewCollection("object_cache:user_settings", variables.StandardTTL)
	Sessions = NewCollection("object_cache:session", variables.StandardTTL)
	Posts = NewCollection("object_cache:post_service", variables.StandardTTL)
	Comments = NewCollection("object_cache:comment", variables.StandardTTL)
	Media = NewCollection("object_cache:media", variables.StandardTTL)
	Messages = NewCollection("object_cache:msg", variables.StandardTTL)
	Conversations = NewCollection("object_cache:conv", variables.StandardTTL)
	Members = NewCollection("object_cache:member", variables.StandardTTL)
	Relations = NewCollection("object_cache:rel", variables.StandardTTL)
	Saved = NewCollection("object_cache:saved:zset", variables.StandardTTL)
	PostComments = NewCollection("object:comments:zset", variables.StandardTTL)
	UserTimeline = NewCollection("user_cache:posts:zset", variables.StandardTTL)
	NotificationsObj = NewCollection("object_cache:notification", variables.StandardTTL)

	// --- SPEED Cache ---
	UsersLite = NewCollection("speed_cache:user:lite", 0)
	UsersLex = NewCollection("speed_cache:user:search", 0)
	ConvMeta = NewCollection("speed_cache:conversation:meta", variables.StandardTTL)
	ConvMembers = NewCollection("speed_cache:conversation:members", variables.StandardTTL)
	SpeedFollowers = NewCollection("speed_cache:followers:default", 0)
	SpeedRelations = NewCollection("speed_cache:relations:default", 0)
	SpeedAddable = NewCollection("speed_cache:addable:default", variables.StandardTTL)

	// --- FEED Cache ---
	FeedsObject = NewCollection("feed:state", variables.StandardTTL)
	FeedsMailbox = NewCollection("feed_cache:mailbox", variables.StandardTTL)
	FeedsPersonalized = NewCollection("feed:personalized", variables.StandardTTL)

	// --- ALGORITHM & MOST Cache ---
	ContentVectors = NewCollection("most_cache:vec", variables.StandardTTL)
	RankedPosts = NewCollection("most_cache", variables.StandardTTL)
	TagPosts = NewCollection("most_cache:idx:tag", variables.StandardTTL)
	TrendGlobalHourly = NewCollection("trend:global:hourly", 0)
	TrendGlobalDaily = NewCollection("trend:global:daily", 0)
	TrendTagDaily = NewCollection("trend:tag:daily", 0)
	TrendTagWeekly = NewCollection("trend:tag:weekly", 0)
	HashtagLeaderboard = NewCollection("hashtag:leaderboard", 0)

	// --- SYSTEM Cache ---
	RateLimits = NewCollection("rate_limit:ip", 10*time.Second)
	DLQ = NewCollection("dlq", 0)
	GraphEdges = NewCollection("graph_cache:tag_edges", 0)
	Tags = NewCollection("tags", 0)
	HashtagCanon = NewCollection("hashtag:canon", 0)

	// --- INDEX & IDEMPOTENCE ---
	SessionIndexes = NewCollection("session_cache", variables.StandardTTL)
	PostLikesSet = NewCollection("post:likes_set", 0)
	CommentLikesSet = NewCollection("comment:likes_set", 0)
	SystemStatus = NewCollection("system:status", 0)

	// --- CUCKOO FILTER ---
	CuckooSeen = NewCollection("cuckoo:seen", variables.StandardTTL)

	// --- MESSAGING & LSH ---
	ConvParticipants = NewCollection("conv:participants", 0)
	UserInbox = NewCollection("inbox:user", 0)
	MessagesIndex = NewCollection("messages:idx", 24*time.Hour)
	LSHBuckets = NewCollection("lsh:bucket", variables.StandardTTL)

	// --- ACTIVITY FEED ---
	FeedSchedule = NewCollection("feed:precompute:schedule", 0)
	NotificationsZSet = NewCollection("notifications:user", variables.StandardTTL)
	NotificationCursors = NewCollection("notifications:cursor", 0)
	InboxActivity = NewCollection("inbox:activity", 0)

	// --- TELEMETRY Cache ---
	TelemetryVectors = NewCollection("telemetry:vectors", variables.StandardTTL)
	TelemetryTags = NewCollection("telemetry:tags", variables.StandardTTL)
	TelemetryTimestamps = NewCollection("telemetry:timestamps", variables.StandardTTL)

	// --- WEBSOCKET Cache ---
	Presence = NewCollection("presence:user", 60*time.Second)

	// --- PUB/SUB CHANNELS & WORKER QUEUES ---
	ChannelUser = NewCollection("channel:user", 0)
	ChannelCommunity = NewCollection("channel:community", 0)
	WorkerQueue = NewCollection("worker:queue", 0)
}

// ============================================================================
// 3. INFRASTRUCTURE & CYCLE DE VIE
// ============================================================================

// IsReady isole l'état de l'infrastructure pour les routines de maintenance.
func IsReady() bool {
	return redisgo.Rdb != nil
}

// Keys renvoie toutes les clés correspondant à un pattern global.
func Keys(ctx context.Context, pattern string) ([]string, error) {
	return redisgo.Rdb.Keys(ctx, pattern).Result()
}

// FlushDB vide l'intégralité de la base de données Redis courante.
func FlushDB(ctx context.Context) error {
	return redisgo.Rdb.FlushDB(ctx).Err()
}

// ============================================================================
// 4. STRUCTURE DE COLLECTION (Le Cœur du DDD)
// ============================================================================

type Collection struct {
	Prefix     string        // ex: "post_service" (donnera "post_service:123")
	Client     *redis.Client // Client Redis
	DefaultTTL time.Duration // Durée de vie par défaut
}

func NewCollection(prefix string, ttl time.Duration) *Collection {
	return &Collection{
		Prefix:     prefix,
		Client:     redisgo.Rdb,
		DefaultTTL: ttl,
	}
}

// Key gère la construction stricte de la clé Redis finale : "prefix:id"
func (c *Collection) Key(id any) string {
	return fmt.Sprintf("%s:%v", c.Prefix, id)
}

// Exists vérifie rapidement si la clé d'une collection existe (O(1)).
func (c *Collection) Exists(ctx context.Context, id any) (bool, error) {
	count, err := c.Client.Exists(ctx, c.Key(id)).Result()
	return count > 0, err
}

// ============================================================================
// 5. PRIMITIVES OBJET (JSON / MsgPack)
// ============================================================================

// SetObject stocke une struct Go en MsgPack dans Redis avec le TTL par défaut.
func (c *Collection) SetObject(ctx context.Context, id any, data any) error {
	msgpackBytes, err := msgpack.Marshal(data)
	if err != nil {
		return fmt.Errorf("redis marshal nubo_error: %w", err)
	}
	return c.Client.Set(ctx, c.Key(id), msgpackBytes, c.DefaultTTL).Err()
}

// GetObject récupère un objet et le désérialise dans 'dest'.
func (c *Collection) GetObject(ctx context.Context, id any, dest any) error {
	val, err := c.Client.Get(ctx, c.Key(id)).Bytes()
	if err != nil {
		return err // redis.Nil si absent
	}
	return msgpack.Unmarshal(val, dest)
}

// DeleteObject supprime un objet du cache.
func (c *Collection) DeleteObject(ctx context.Context, id any) error {
	return c.Client.Del(ctx, c.Key(id)).Err()
}

// RefreshTTL prolonge la durée de vie d'un objet (utile pour les sessions).
func (c *Collection) RefreshTTL(ctx context.Context, id any) error {
	return c.Client.Expire(ctx, c.Key(id), c.DefaultTTL).Err()
}

// ============================================================================
// 6. PRIMITIVES BRUTES (Primitives & Entiers)
// ============================================================================

// SetPrimitive stocke une valeur brute sans MsgPack (utile pour les index).
func (c *Collection) SetPrimitive(ctx context.Context, id any, val any) error {
	return c.Client.Set(ctx, c.Key(id), val, c.DefaultTTL).Err()
}

// DeletePrimitive supprime une valeur primitive brute.
func (c *Collection) DeletePrimitive(ctx context.Context, id any) error {
	return c.Client.Del(ctx, c.Key(id)).Err()
}

// GetInt64 récupère une valeur primitive brute sous forme d'entier.
func (c *Collection) GetInt64(ctx context.Context, id any) (int64, error) {
	return c.Client.Get(ctx, c.Key(id)).Int64()
}

// Incr incrémente une valeur numérique et retourne le nouveau compteur.
func (c *Collection) Incr(ctx context.Context, id any) (int64, error) {
	return c.Client.Incr(ctx, c.Key(id)).Result()
}

// ============================================================================
// 7. PRIMITIVES HASH
// ============================================================================

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

// ============================================================================
// 8. PRIMITIVES SET & LIST
// ============================================================================

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

func (c *Collection) SAddCount(ctx context.Context, id any, members ...any) (int64, error) {
	return c.Client.SAdd(ctx, c.Key(id), members...).Result()
}

func (c *Collection) SRemCount(ctx context.Context, id any, members ...any) (int64, error) {
	return c.Client.SRem(ctx, c.Key(id), members...).Result()
}

func (c *Collection) LPush(ctx context.Context, id any, values ...any) error {
	return c.Client.LPush(ctx, c.Key(id), values...).Err()
}

// ============================================================================
// 9. BULK OPERATIONS & PIPELINE
// ============================================================================

func (c *Collection) Pipeline() redis.Pipeliner {
	return c.Client.Pipeline()
}

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

type GetManyResult struct {
	Found      map[int64][]byte
	MissingIDs []int64
}

// GetMany récupère une liste d'objets en un seul appel réseau (MGET).
func (c *Collection) GetMany(ctx context.Context, ids []int64) (*GetManyResult, error) {
	if len(ids) == 0 {
		return &GetManyResult{}, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = c.Key(id)
	}

	values, err := c.Client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := &GetManyResult{
		Found:      make(map[int64][]byte),
		MissingIDs: make([]int64, 0),
	}

	for i, val := range values {
		originalID := ids[i]
		if val == nil {
			result.MissingIDs = append(result.MissingIDs, originalID)
		} else {
			if strVal, ok := val.(string); ok {
				result.Found[originalID] = []byte(strVal)
			} else {
				result.MissingIDs = append(result.MissingIDs, originalID)
			}
		}
	}
	return result, nil
}

// ============================================================================
// 10. CUCKOO FILTER (RedisBloom)
// ============================================================================

func (c *Collection) CFExists(ctx context.Context, id any, item any) (bool, error) {
	return c.Client.Do(ctx, "CF.EXISTS", c.Key(id), item).Bool()
}

func (c *Collection) CFAdd(ctx context.Context, id any, item any) error {
	return c.Client.Do(ctx, "CF.ADD", c.Key(id), item).Err()
}

// ============================================================================
// 11. PUB/SUB FLUX (Pattern Claim Check)
// ============================================================================

const DefaultFluxTTL = 5 * time.Second

func PushFluxWithTTL(rdb *redis.Client, nodeName string, messageID string, message []byte, ttl time.Duration) error {
	ctx := context.Background()
	key := "fluxmsg:" + messageID
	if err := rdb.Set(ctx, key, message, ttl).Err(); err != nil {
		return err
	}
	channel := "flux:" + nodeName
	if err := rdb.Publish(ctx, channel, messageID).Err(); err != nil {
		return err
	}
	return nil
}

func SubscribeFlux(rdb *redis.Client, nodeName string) (<-chan []byte, context.CancelFunc) {
	channel := "flux:" + nodeName
	ctx, cancel := context.WithCancel(context.Background())
	pubsub := rdb.Subscribe(ctx, channel)
	ch := make(chan []byte, 100)

	go func() {
		defer func(pubsub *redis.PubSub) {
			err := pubsub.Close()
			if err != nil {
				log.Printf("Erreur fermeture pubsub: %v", err)
			}
		}(pubsub)
		defer close(ch)

		for msg := range pubsub.Channel() {
			messageID := msg.Payload
			readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
			data, err := rdb.Get(readCtx, "fluxmsg:"+messageID).Bytes()
			readCancel()

			if errors.Is(redis.Nil, err) {
				continue
			} else if err != nil {
				log.Printf("  Erreur flux %s : impossible de lire %s : %v", nodeName, messageID, err)
				continue
			}

			select {
			case ch <- data:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, cancel
}

// ============================================================================
// 12. PUB/SUB (Mode DDD strict)
// ============================================================================

// Publish publie un message sur un canal Pub/Sub
func (c *Collection) Publish(ctx context.Context, id any, message any) error {
	return c.Client.Publish(ctx, c.Key(id), message).Err()
}

// PublishMultiple publie un même message sur plusieurs canaux via un Pipeline (O(1) RTT)
func (c *Collection) PublishMultiple(ctx context.Context, ids []int64, message any) error {
	if len(ids) == 0 {
		return nil
	}
	pipe := c.Client.Pipeline()
	for _, id := range ids {
		pipe.Publish(ctx, c.Key(id), message)
	}
	_, err := pipe.Exec(ctx)
	return err
}
