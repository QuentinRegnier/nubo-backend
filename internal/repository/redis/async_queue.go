package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
)

// --- CONSTANTES ---

type ActionType string

const (
	ActionCreate ActionType = "ADD" // J'utilise tes termes "ADD"
	ActionUpdate ActionType = "UPD"
	ActionDelete ActionType = "DEL"
)

// EntityType : Sur quoi on agit
type EntityType string

const (
	EntityUser         EntityType = "Users"
	EntityUserSettings EntityType = "UserSettings"
	EntitySession      EntityType = "Sessions"
	EntityRelation     EntityType = "Relations"
	EntityPost         EntityType = "Posts"
	EntityComment      EntityType = "Comments"
	EntityLike         EntityType = "Likes"
	EntityMedia        EntityType = "Media"
	EntityConversation EntityType = "Conversations"
	EntityMembers      EntityType = "Members"
	EntityMessage      EntityType = "Messages"
	// Ajoute les autres entités selon tes besoins
)

// DBTarget : Bitmask pour savoir où envoyer (Mongo, Postgres, ou les deux)
type DBTarget int

const (
	TargetNone     DBTarget = 0
	TargetMongo    DBTarget = 1 << 0 // 1
	TargetPostgres DBTarget = 1 << 1 // 2
	TargetAll      DBTarget = TargetMongo | TargetPostgres
)

// --- STRUCTURE DE L'EVENT ---

type AsyncEvent struct {
	ID        int64      `json:"id"`
	Type      EntityType `json:"t"`
	Action    ActionType `json:"a"`
	Payload   any        `json:"p"`
	Targets   DBTarget   `json:"tg"`
	Timestamp int64      `json:"ts"` // Timestamp UnixMilli
}

// --- QUEUE MANAGER ---

const (
	QueueShards = 64

	// QueueBasePrefix est le préfixe utilisé pour séparer les files par Type et Action.
	// Format de la liste : q:{shardID}:{Type}:{Action}
	QueueBasePrefix = "q:"

	// StatsBasePrefix est le préfixe utilisé pour les métriques du Dashboard.
	// Format : h:stats:{shardID}
	StatsBasePrefix = "h:stats:"
)

// EnqueueDB : Ajout du paramètre partitionKey (int64)
// Si partitionKey est 0, on utilise l'ID de l'objet pour choisir la file.
// Si partitionKey est fourni (ex: ID du User), on utilise ça pour forcer la file.
func EnqueueDB(ctx context.Context, id int64, partitionKey int64, entity EntityType, action ActionType, data interface{}, target DBTarget) error {

	now := time.Now().UnixMilli()
	event := AsyncEvent{
		ID:        id,
		Type:      entity,
		Action:    action,
		Payload:   data,
		Targets:   target,
		Timestamp: now,
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	// C'EST ICI QUE TOUT SE JOUE : Choix du Shard
	keyForSharding := id
	if partitionKey != 0 {
		keyForSharding = partitionKey
	}
	shardID := getShardID(keyForSharding)

	shardStr := strconv.Itoa(int(shardID))
	queueKey := fmt.Sprintf("%s%s:%s:%s", QueueBasePrefix, shardStr, entity, action)
	statsKey := StatsBasePrefix + shardStr
	countField := fmt.Sprintf("%s:%s:count", entity, action)
	tsField := fmt.Sprintf("%s:%s:ts", entity, action)

	pipe := redis.Rdb.Pipeline()
	pipe.RPush(ctx, queueKey, bytes)
	pipe.HIncrBy(ctx, statsKey, countField, 1)
	pipe.HSetNX(ctx, statsKey, tsField, now)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline failed: %w", err)
	}

	return nil
}

// --- OUTILS POUR LE WORKER INTELLIGENT ---

// QueueStats représente une ligne du tableau de bord
type QueueStats struct {
	Type     EntityType
	Action   ActionType
	Count    int64
	OldestTS int64
	Delay    time.Duration // Calculé (Now - OldestTS)
}

// GetShardStats récupère tout le tableau de bord pour un shard donné
func GetShardStats(ctx context.Context, shardID int) ([]QueueStats, error) {
	statsKey := StatsBasePrefix + strconv.Itoa(shardID)

	// Récupère tout le Hash (HGETALL)
	rawMap, err := redis.Rdb.HGetAll(ctx, statsKey).Result()
	if err != nil {
		return nil, err
	}

	// Parsing des résultats pour construire notre "Vision"
	// On regroupe count et ts par clé
	tempMap := make(map[string]*QueueStats)

	now := time.Now().UnixMilli()

	for field, val := range rawMap {
		// field est genre "MSG:ADD:count" ou "MSG:ADD:ts"
		parts := strings.Split(field, ":")
		if len(parts) != 3 {
			continue
		}

		key := parts[0] + ":" + parts[1] // "MSG:ADD"

		if _, exists := tempMap[key]; !exists {
			tempMap[key] = &QueueStats{
				Type:   EntityType(parts[0]),
				Action: ActionType(parts[1]),
			}
		}

		vInt, _ := strconv.ParseInt(val, 10, 64)
		switch parts[2] {
		case "count":
			tempMap[key].Count = vInt
		case "ts":
			tempMap[key].OldestTS = vInt
			// Calcul du délai en direct
			tempMap[key].Delay = time.Duration(now-vInt) * time.Millisecond
		}
	}

	// Conversion en slice propre
	var stats []QueueStats
	for _, s := range tempMap {
		if s.Count > 0 { // On ne garde que ce qui a du travail
			stats = append(stats, *s)
		}
	}
	return stats, nil
}

// PopSmartBatch récupère un batch précis choisi par le Worker
// Il met aussi à jour le Dashboard (décrémente count, met à jour ts)
func PopSmartBatch(ctx context.Context, shardID int, entity EntityType, action ActionType, batchSize int64) ([]AsyncEvent, error) {
	shardStr := strconv.Itoa(shardID)
	queueKey := fmt.Sprintf("%s%s:%s:%s", QueueBasePrefix, shardStr, entity, action)
	statsKey := StatsBasePrefix + shardStr

	// 1. Récupérer les éléments (LPOP count)
	results, err := redis.Rdb.LPopCount(ctx, queueKey, int(batchSize)).Result()
	if err != nil { // Redis.Nil si vide
		return nil, nil
	}
	countPopped := int64(len(results))
	if countPopped == 0 {
		return nil, nil
	}

	// 2. Désérialiser avec PROTECTION DES INT64
	events := make([]AsyncEvent, 0, countPopped)
	for _, res := range results {
		var evt AsyncEvent

		// --- CORRECTION ICI ---
		// Au lieu de json.Unmarshal, on utilise Decoder + UseNumber()
		decoder := json.NewDecoder(strings.NewReader(res))
		decoder.UseNumber() // <--- C'est la ligne magique !

		if err := decoder.Decode(&evt); err == nil {
			events = append(events, evt)
		} else {
			// Optionnel : Loguer l'erreur si le JSON est pourri
			fmt.Printf("❌ Erreur décodage event queue: %v\n", err)
		}
		// ----------------------
	}

	// 3. Mise à jour du Dashboard
	go updateDashboardAfterPop(context.Background(), statsKey, queueKey, string(entity), string(action), countPopped)

	return events, nil
}

// updateDashboardAfterPop gère la cohérence du tableau de bord (Async pour perf)
func updateDashboardAfterPop(ctx context.Context, statsKey, queueKey, entity, action string, poppedCnt int64) {
	countField := fmt.Sprintf("%s:%s:count", entity, action)
	tsField := fmt.Sprintf("%s:%s:ts", entity, action)

	pipe := redis.Rdb.Pipeline()

	// Décrémente le compteur
	pipe.HIncrBy(ctx, statsKey, countField, -poppedCnt)

	// Pour mettre à jour le Timestamp du plus vieux, on doit "Peeker" le nouvel élément de tête
	// LINDEX key 0 nous donne le premier élément sans le supprimer
	lindexCmd := pipe.LIndex(ctx, queueKey, 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return
	}

	// Analyse du résultat LINDEX
	nextHeadVal, err := lindexCmd.Result()
	if err == nil && nextHeadVal != "" {
		// Il reste des éléments ! On lit le timestamp du nouveau chef de file
		var nextEvt AsyncEvent
		if json.Unmarshal([]byte(nextHeadVal), &nextEvt) == nil {
			// Mise à jour du timestamp "oldest"
			redis.Rdb.HSet(ctx, statsKey, tsField, nextEvt.Timestamp)
		}
	} else {
		// La liste est vide ou erreur -> on nettoie le timestamp car il n'y a plus de "vieux"
		redis.Rdb.HDel(ctx, statsKey, tsField)
		// On peut aussi remettre le count à 0 par sécurité
		redis.Rdb.HSet(ctx, statsKey, countField, 0)
	}
}

// getShardID (Inchangé)
func getShardID(id int64) uint32 {
	idStr := strconv.FormatInt(id, 10)
	return crc32.ChecksumIEEE([]byte(idStr)) % QueueShards
}
