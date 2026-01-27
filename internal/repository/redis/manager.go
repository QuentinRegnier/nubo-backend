package redis

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/go-redis/redis/v8"
)

// ---------------- Initialisation ----------------
// Permet au Sentinel de retrouver la collection par son nom string
var (
	collectionRegistry = make(map[string]*Collection)
	registryMu         sync.RWMutex
)

// declarations globales
var (
	Users         *Collection
	UserSettings  *Collection
	Sessions      *Collection
	Relations     *Collection
	Posts         *Collection
	Comments      *Collection
	Likes         *Collection
	Media         *Collection
	Conversations *Collection
	Members       *Collection
	Messages      *Collection
)

// InitCacheDatabase initialise la structure logique de Redis pour les caches
func InitCacheDatabase() {
	// Initialiser les collections

	schemaUsers := domain.UsersSchema
	schemaUserSettings := domain.UserSettingsSchema
	schemaSessions := domain.SessionsSchema
	schemaRelations := domain.RelationsSchema
	schemaPosts := domain.PostsSchema
	schemaComments := domain.CommentsSchema
	schemaLikes := domain.LikesSchema
	schemaMedia := domain.MediaSchema
	schemaConversations := domain.ConversationsSchema
	schemaMembers := domain.MembersSchema
	schemaMessages := domain.MessagesSchema

	// variables globales
	// MODIFICATION : On définit qui est permanent (false) et qui est évictable (true)

	// Données CRITIQUES (Pas de suppression auto)
	Users = NewCollection("users", schemaUsers, redisgo.Rdb, false)
	UserSettings = NewCollection("user_settings", schemaUserSettings, redisgo.Rdb, false)
	Sessions = NewCollection("sessions", schemaSessions, redisgo.Rdb, false)

	// Données EVICTABLES (Suppression si RAM pleine)
	Posts = NewCollection("posts", schemaPosts, redisgo.Rdb, true)
	Comments = NewCollection("comments", schemaComments, redisgo.Rdb, true)
	Likes = NewCollection("likes", schemaLikes, redisgo.Rdb, true)
	Media = NewCollection("media", schemaMedia, redisgo.Rdb, true)
	Conversations = NewCollection("conversations", schemaConversations, redisgo.Rdb, true)
	Members = NewCollection("members", schemaMembers, redisgo.Rdb, true)
	Messages = NewCollection("messages", schemaMessages, redisgo.Rdb, true)
	Relations = NewCollection("relations", schemaRelations, redisgo.Rdb, true)

	log.Println("Structure Redis (caches) initialisée")
}

// ---------------- Collection et schéma ----------------

type Collection struct {
	Name        string                  // ex: "messages"
	Schema      map[string]reflect.Kind // ex: {"id": reflect.Int, "content": reflect.String}
	Redis       *redis.Client
	IsEvictable bool
	Expiration  time.Duration // TTL par défaut pour chaque élément, facultatif
}

// NewCollection crée une collection avec un schéma et LRU optionnel
func NewCollection(name string, schema map[string]reflect.Kind, rdb *redis.Client, isEvictable bool) *Collection {
	_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Initialiser les indexs pour chaque champ du schéma
	for field := range schema {
		if field == "id" {
			continue
		}
		// on ne crée pas les valeurs ici (elles seront ajoutées au fur et à mesure)
		// mais on garde la structure logique
		log.Printf("Index initialisé pour collection=%s, champ=%s", name, field)
	}

	c := &Collection{
		Name:        name,
		Schema:      schema,
		Redis:       rdb,
		IsEvictable: isEvictable,
	}

	// Enregistrement dans le registre pour le Sentinel
	registryMu.Lock()
	collectionRegistry[name] = c
	registryMu.Unlock()

	return c
}

// ---------------- Validation ----------------

func (c *Collection) validate(obj map[string]any) error {
	for field, kind := range c.Schema {
		val, ok := obj[field]
		if !ok {
			return fmt.Errorf("champ manquant: %s", field)
		}

		// Récupération du type réel
		actualKind := reflect.TypeOf(val).Kind()

		if actualKind != kind {
			// --- CORRECTION REDIS ICI ---
			// Si Redis attend une String (JSON) mais que le schéma dit Slice/Map/Struct, on accepte !
			isJsonSerialized := actualKind == reflect.String && (kind == reflect.Slice || kind == reflect.Map || kind == reflect.Struct)

			if !isJsonSerialized {
				return fmt.Errorf("champ %s doit être de type %s (reçu %s)", field, kind.String(), actualKind.String())
			}
		}
	}
	return nil
}

// ---------------- Set ----------------

// Set ajoute un élément dans la collection avec gestion automatique ZSET/SET
func (c *Collection) Set(ctx context.Context, obj map[string]any) error {
	if err := c.validate(obj); err != nil {
		log.Println("Validation échouée:", err)
		return err
	}

	id := fmt.Sprintf("%v", obj["id"])
	objKey := "cache:" + c.Name + ":" + id

	// 1. Sauvegarde complète
	if err := c.Redis.HMSet(ctx, objKey, obj).Err(); err != nil {
		return err
	}

	// 2. Indexation
	for field, kind := range c.Schema {
		if field == "id" {
			continue
		}
		val, ok := obj[field]
		if !ok {
			continue
		}

		// Détection ZSET (Numérique ou Date)
		isZSet := shouldIndexAsZSet(field, kind)
		var score float64

		if isZSet {
			// On calcule le score seulement si c'est nécessaire
			if field == "created_at" || field == "updated_at" || field == "joined_at" || field == "expires_at" || field == "birthdate" || field == "ban_expires_at" || field == "tolerance_time" {
				if t, err := parseToTime(val); err == nil {
					score = float64(t.Unix())
				}
			} else {
				if n, err := toInt64(val); err == nil {
					score = float64(n)
				}
			}
			// Indexation par Score (ZSET)
			// ex: idx:zset:users:age -> score=25, member=ID
			idxKey := fmt.Sprintf("idx:zset:%s:%s", c.Name, field)
			c.Redis.ZAdd(ctx, idxKey, &redis.Z{
				Score:  score,
				Member: id,
			})
		} else {
			// Indexation par Valeur Exacte (SET)
			// ex: idx:users:role:admin -> member=ID
			valStr := fmt.Sprintf("%v", val)
			idxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, valStr)
			c.Redis.SAdd(ctx, idxKey, id)
		}
	}

	// Mise à jour LRU Distribué (ZADD)
	if c.IsEvictable {
		// Score = Maintenant, Member = "collection:id"
		member := c.Name + ":" + id
		c.Redis.ZAdd(ctx, "idx:lru:global", &redis.Z{
			Score:  float64(time.Now().UnixNano()),
			Member: member,
		})
	}

	return nil
}

// ---------------- Get ----------------

// Get retourne tous les éléments correspondant au filtre (MongoDB-like)
func (c *Collection) Get(ctx context.Context, filter map[string]any) ([]map[string]any, error) {
	// 1. Récupérer l’ensemble des IDs possibles via evalTree
	fmt.Printf("\n🕵️ --- DEBUG MANAGER GET START [%s] ---\n", c.Name)
	fmt.Printf("📥 Filtres reçus: %+v\n", filter)

	candidateSet, _, err := evalTree(ctx, c.Redis, c.Name, filter, "")
	if err != nil {
		return nil, err
	}

	fmt.Printf("🔍 Les IDs trouvés: %+v\n", candidateSet)

	// 2. Charger les objets correspondants
	results := []map[string]any{}
	for id := range candidateSet {
		// fmt.Printf("➡️ Chargement ID=%s\n", id) // (Optionnel: tu peux retirer les logs verbeux maintenant)
		objKey := "cache:" + c.Name + ":" + id

		// HGetAll renvoie map[string]string ! Tout est string !
		data, err := c.Redis.HGetAll(ctx, objKey).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		obj := make(map[string]any)

		// --- CORRECTION TYPAGE ICI ---
		for k, vStr := range data {
			// 1. Gestion spéciale de l'ID (qui est souvent un int64/snowflake)
			if k == "id" {
				if n, err := strconv.ParseInt(vStr, 10, 64); err == nil {
					obj[k] = n // On stocke un vrai int64
				} else {
					obj[k] = vStr // Fallback string si échec
				}
				continue
			}

			// 2. On regarde le Schéma pour savoir comment convertir
			kind, known := c.Schema[k]
			if !known {
				// Champ inconnu dans le schéma : on garde la string brute
				obj[k] = vStr
				continue
			}

			switch kind {
			// Cas Numériques
			case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
				if n, err := strconv.ParseInt(vStr, 10, 64); err == nil {
					// Attention: mapstructure préfère souvent int64 ou int, ça dépend de ta struct
					obj[k] = n
				} else {
					obj[k] = vStr
				}

			case reflect.Uint, reflect.Uint64, reflect.Uint32:
				if n, err := strconv.ParseUint(vStr, 10, 64); err == nil {
					obj[k] = n
				} else {
					obj[k] = vStr
				}

			case reflect.Float64, reflect.Float32:
				if f, err := strconv.ParseFloat(vStr, 64); err == nil {
					obj[k] = f
				} else {
					obj[k] = vStr
				}

			// Cas Booléens (Redis stocke souvent "0" ou "1", ou "true"/"false")
			case reflect.Bool:
				if b, err := strconv.ParseBool(vStr); err == nil {
					obj[k] = b
				} else if vStr == "1" {
					obj[k] = true
				} else if vStr == "0" {
					obj[k] = false
				} else {
					obj[k] = vStr
				}

			// Cas Dates (détection par nom ou type struct si ta logique le permet)
			// Ta fonction 'parseToTime' est parfaite pour ça
			default:
				// Si c'est un champ date connu
				if k == "created_at" || k == "updated_at" || k == "joined_at" || k == "expires_at" || k == "birthdate" || k == "ban_expires_at" || k == "tolerance_time" {
					if t, err := parseToTime(vStr); err == nil {
						obj[k] = t
					} else {
						obj[k] = vStr
					}
				} else {
					// String standard
					obj[k] = vStr
				}
			}
		}
		// --- FIN CORRECTION ---

		results = append(results, obj)

		// ... (Code LRU inchangé)
	}

	return results, nil
}

// ----------- Delete ----------------

// Delete supprime les éléments et nettoie les index (SET et ZSET)
func (c *Collection) Delete(ctx context.Context, filter map[string]any) error {
	objs, err := c.Get(ctx, filter)
	if err != nil {
		return err
	}

	pipe := c.Redis.TxPipeline()

	for _, obj := range objs {
		id := fmt.Sprintf("%v", obj["id"])
		objKey := "cache:" + c.Name + ":" + id

		// Supprimer l'objet
		pipe.Del(ctx, objKey)

		// Nettoyer les indexs
		for field, kind := range c.Schema {
			if field == "id" {
				continue
			}

			// Vérifier si c'était un ZSET ou un SET (même logique que Set)
			val, ok := obj[field]
			if !ok {
				continue
			}

			// UTILISATION DU HELPER CENTRALISÉ
			isZSet := shouldIndexAsZSet(field, kind)

			if isZSet {
				// Suppression dans ZSET
				idxKey := fmt.Sprintf("idx:zset:%s:%s", c.Name, field)
				pipe.ZRem(ctx, idxKey, id)
			} else {
				// Suppression dans SET
				valStr := fmt.Sprintf("%v", val)
				idxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, valStr)
				pipe.SRem(ctx, idxKey, id)

				// Note: Tu avais une logique pour supprimer les clés vides après,
				// tu peux la garder si tu veux, ici je simplifie pour la clarté.
			}
		}

		// Nettoyage du LRU global pour ne pas laisser de fantômes
		if c.IsEvictable {
			member := c.Name + ":" + id
			pipe.ZRem(ctx, "idx:lru:global", member)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

// ---------------- Update ----------------

// Update met à jour les éléments et gère proprement la rotation des index
func (c *Collection) Update(ctx context.Context, filter map[string]interface{}, update map[string]interface{}) error {

	// 1. Récupérer les objets cibles
	objs, err := c.Get(ctx, filter)
	if err != nil {
		return err
	}

	pipe := c.Redis.TxPipeline()

	for _, obj := range objs {
		id := fmt.Sprintf("%v", obj["id"])
		objKey := "cache:" + c.Name + ":" + id

		// 2. Itérer sur les champs à modifier
		for field, newVal := range update {
			if field == "id" {
				continue
			}

			// --- ÉTAPE CRUCIALE : Récupérer l'ancienne valeur AVANT modif ---
			oldVal, hasOld := obj[field]

			// Déterminer le type d'index (ZSET ou SET)
			kind := c.Schema[field]
			// UTILISATION DU HELPER CENTRALISÉ
			isZSet := shouldIndexAsZSet(field, kind)
			var newScore float64

			if isZSet {
				// Calcul du nouveau score si c'est un ZSET
				if field == "created_at" || field == "updated_at" || field == "joined_at" || field == "expires_at" || field == "tolerance_time" || field == "birthdate" || field == "ban_expires_at" {
					if t, err := parseToTime(newVal); err == nil {
						newScore = float64(t.Unix())
					}
				} else {
					if n, err := toInt64(newVal); err == nil {
						newScore = float64(n)
					}
				}
			}

			// --- GESTION DES INDEX ---

			if isZSet {
				// CAS ZSET (Score)
				// Redis gère l'update atomique : ZADD écrase l'ancien score pour ce membre.
				// Pas besoin de ZREM explicite sur la même clé.
				idxKey := fmt.Sprintf("idx:zset:%s:%s", c.Name, field)
				pipe.ZAdd(ctx, idxKey, &redis.Z{
					Score:  newScore,
					Member: id,
				})
			} else {
				// CAS SET (Valeurs distinctes, ex: username, role)
				// IL FAUT SUPPRIMER L'ANCIENNE ENTRÉE
				if hasOld {
					oldValStr := fmt.Sprintf("%v", oldVal)
					oldIdxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, oldValStr)
					pipe.SRem(ctx, oldIdxKey, id) // <-- SUPPRESSION ICI
				}

				// ET AJOUTER LA NOUVELLE
				newValStr := fmt.Sprintf("%v", newVal)
				newIdxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, newValStr)
				pipe.SAdd(ctx, newIdxKey, id)
			}

			// 3. Mettre à jour l'objet en mémoire pour le HMSet final
			obj[field] = newVal
		}

		// 4. Sauvegarder l'objet complet mis à jour
		pipe.HMSet(ctx, objKey, obj)

		// 5. Mise à jour LRU Distribué
		if c.IsEvictable {
			member := c.Name + ":" + id
			// Note: On pourrait utiliser pipe.ZAdd, mais attention au contexte
			// Pour l'instant on laisse en appel direct ou on l'ajoute au pipe existant
			// Si tu veux l'ajouter au pipe (recommandé) :
			pipe.ZAdd(ctx, "idx:lru:global", &redis.Z{
				Score:  float64(time.Now().UnixNano()),
				Member: member,
			})
		}
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Printf("Erreur execution pipeline Update: %v", err)
		return err
	}

	return nil
}

// ---------------- Filtrage ----------------

func evalTree(ctx context.Context, rdb *redis.Client, collName string, filter map[string]any, type_before string) (map[string]struct{}, []map[string]any, error) {
	// Cas 1 : opérateurs logiques
	fmt.Printf("🔎 evalTree called with filter: %+v\n", filter)
	if orOps, ok := filter["$or"]; ok {
		arr, _ := orOps.([]any)
		unionSet := make(map[string]struct{})
		for _, sub := range arr {
			subFilter, _ := sub.(map[string]any)
			res, _, err := evalTree(ctx, rdb, collName, subFilter, "$or")
			if err != nil {
				return nil, nil, err
			}
			for id := range res {
				unionSet[id] = struct{}{}
			}
		}
		return unionSet, nil, nil
	}

	if andOps, ok := filter["$and"]; ok {
		arr, _ := andOps.([]any)
		var interSet map[string]struct{}
		var del = []map[string]any{}
		for _, sub := range arr {
			subFilter, _ := sub.(map[string]any)
			res, del, err := evalTree(ctx, rdb, collName, subFilter, "$and")
			if err != nil {
				return nil, del, err
			}
			if interSet == nil {
				interSet = res
			} else {
				for id := range interSet {
					if _, ok := res[id]; !ok {
						delete(interSet, id)
					}
				}
			}
		}
		if interSet == nil {
			interSet = make(map[string]struct{})
		}

		for _, d := range del {
			for _, raw := range d {
				cond, _ := raw.(map[string]any)
				ids := []string{}
				for id := range interSet {
					ids = append(ids, id)
				}
				deleteIDsFromCondition(cond, &ids)
				// reconstruire interSet
				newSet := make(map[string]struct{})
				for _, id := range ids {
					newSet[id] = struct{}{}
				}
				interSet = newSet
			}
		}
		return interSet, nil, nil
	}

	// Cas 2 : feuille = condition COND
	// Exemple : { "conversation_id": { "$eq": "22" } }
	resultSet := make(map[string]struct{})
	del := []map[string]any{}
	for field, raw := range filter {
		if field == "id" && type_before == "$and" {
			cond, _ := raw.(map[string]any)
			del = append(del, map[string]any{field: cond})
			continue
		}
		cond, _ := raw.(map[string]any)
		for op, val := range cond {
			fmt.Printf("Evaluating condition on field=%s, op=%s, val=%v\n", field, op, val)
			ids, err := fetchIDsForCondition(ctx, rdb, collName, field, op, val)
			fmt.Printf("Fetched IDs for condition: %+v\n", ids)
			if err != nil {
				return nil, nil, err
			}
			for _, id := range ids {
				resultSet[id] = struct{}{}
			}
		}
	}
	return resultSet, del, nil
}

// fetchIDsForCondition : récupère les IDs directement depuis Redis
func fetchIDsForCondition(ctx context.Context, rdb *redis.Client, collName, field, op string, val any) ([]string, error) {
	basePrefix := "idx"

	// 1. Récupérer le schéma pour savoir VRAIMENT comment c'est stocké (ZSET vs SET)
	registryMu.RLock()
	coll, exists := collectionRegistry[collName]
	registryMu.RUnlock()

	isZSetStorage := false
	if exists {
		kind := coll.Schema[field]
		// UTILISATION DU HELPER CENTRALISÉ
		// Il va renvoyer 'false' pour user_id, donc on cherchera dans le bon SET !
		isZSetStorage = shouldIndexAsZSet(field, kind)
	}

	// 2. Construire la clé correcte
	var key string
	if isZSetStorage {
		// La clé ZSET ne contient PAS la valeur, juste le nom du champ
		key = fmt.Sprintf("%s:zset:%s:%s", basePrefix, collName, field)
	} else {
		// La clé SET contiendra la valeur plus tard (concaténée)
		key = fmt.Sprintf("%s:%s:%s", basePrefix, collName, field)
	}
	fmt.Printf("Using key='%s' for field='%s' (isZSet=%v)\n", key, field, isZSetStorage)
	switch op {
	case "$eq":
		if field == "id" {
			idStr := fmt.Sprintf("%v", val)
			objKey := fmt.Sprintf("cache:%s:%s", collName, idStr)
			exists, _ := rdb.Exists(ctx, objKey).Result()
			if exists == 1 {
				return []string{idStr}, nil
			}
			return []string{}, nil
		}

		// --- CORRECTION MAJEURE ICI ---
		if isZSetStorage {
			// Si c'est stocké en ZSET (ex: user_id), $eq devient un Range [val, val]
			score, err := valToScore(val) // Helper function (voir plus bas)
			if err != nil {
				return nil, err
			}
			// On cherche exactement ce score (Min=Score, Max=Score)
			scoreStr := fmt.Sprintf("%f", score)
			rBox := &redis.ZRangeBy{Min: scoreStr, Max: scoreStr}
			return rdb.ZRangeByScore(ctx, key, rBox).Result()
		} else {
			// Si c'est un SET classique (ex: email, token)
			member := fmt.Sprintf("%v", val)
			return rdb.SMembers(ctx, key+":"+member).Result()
		}

	case "$in":
		vals, _ := val.([]any)
		var all []string

		if isZSetStorage {
			// Pour un ZSET, $in est une suite de recherches unitaires par score
			for _, v := range vals {
				score, err := valToScore(v)
				if err != nil {
					continue
				}
				scoreStr := fmt.Sprintf("%f", score)
				ids, _ := rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{Min: scoreStr, Max: scoreStr}).Result()
				all = append(all, ids...)
			}
		} else {
			// Pour un SET, on concatène la valeur à la clé
			for _, v := range vals {
				member := fmt.Sprintf("%v", v)
				ids, err := rdb.SMembers(ctx, key+":"+member).Result()
				if err != nil {
					return nil, err
				}
				all = append(all, ids...)
			}
		}
		return all, nil

	case "$gt", "$gte", "$lt", "$lte":
		// Pour les opérateurs de comparaison, c'est forcément du ZSET
		if !isZSetStorage {
			return nil, fmt.Errorf("opérateur %s impossible sur un champ non-numérique/non-date", op)
		}

		score, err := valToScore(val)
		if err != nil {
			return nil, err
		}

		rBox := &redis.ZRangeBy{Min: "-inf", Max: "+inf"}
		scoreStr := fmt.Sprintf("%f", score)

		switch op {
		case "$gt":
			rBox.Min = "(" + scoreStr
		case "$gte":
			rBox.Min = scoreStr
		case "$lt":
			rBox.Max = "(" + scoreStr
		case "$lte":
			rBox.Max = scoreStr
		}

		return rdb.ZRangeByScore(ctx, key, rBox).Result()

	default:
		return []string{}, nil
	}
}

// Petit helper pour convertir n'importe quoi en float64 (Score)
func valToScore(val any) (float64, error) {
	// Est-ce une date string ou time ?
	if t, err := parseToTime(val); err == nil {
		return float64(t.Unix()), nil
	}
	// Est-ce un nombre ?
	return toFloat64(val)
}

func toFloat64(val any) (float64, error) {
	switch v := val.(type) {
	case int, int8, int16, int32, int64:
		n, _ := toInt64(v)
		return float64(n), nil
	case uint, uint8, uint16, uint32, uint64:
		n, _ := toInt64(v)
		return float64(n), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("impossible de convertir %T en float64", val)
	}
}

// deleteIDsFromCondition : supprime les IDs correspondant à une condition simple. exemple : deleteIDsFromCondition("id", {"$lte": 2}, {1,2,3,4,5}) = {3,4,5} pas de redis donc il faut gérer $eq, $in, $gt, $lt, $gte, $lte on envoie un pointeur vers le slice d'ids pour le modifier en place en supprimant avec des for
func deleteIDsFromCondition(cond map[string]any, ids *[]string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Panic dans deleteIDsFromCondition:", r)
		}
	}()

	for op, val := range cond {
		switch op {
		case "$eq":
			valStr := fmt.Sprintf("%v", val)
			newIDs := []string{}
			for _, id := range *ids {
				if id != valStr {
					newIDs = append(newIDs, id)
				}
			}
			*ids = newIDs

		case "$in":
			vals, _ := val.([]any)
			valSet := make(map[string]struct{})
			for _, v := range vals {
				valSet[fmt.Sprintf("%v", v)] = struct{}{}
			}
			newIDs := []string{}
			for _, id := range *ids {
				if _, found := valSet[id]; !found {
					newIDs = append(newIDs, id)
				}
			}
			*ids = newIDs

		case "$gt", "$gte", "$lt", "$lte":
			valInt, err := toInt64(val)
			if err != nil {
				log.Println("Erreur conversion valeur numérique dans deleteIDsFromCondition:", err)
				continue
			}
			newIDs := []string{}
			for _, id := range *ids {
				idInt, err := strconv.ParseInt(id, 10, 64)
				if err != nil {
					continue
				}

				switch op {
				case "$gt":
					if idInt > valInt {
						newIDs = append(newIDs, id)
					}
				case "$gte":
					if idInt >= valInt {
						newIDs = append(newIDs, id)
					}
				case "$lt":
					if idInt < valInt {
						newIDs = append(newIDs, id)
					}
				case "$lte":
					if idInt <= valInt {
						newIDs = append(newIDs, id)
					}
				}
			}
			*ids = newIDs

		default:
			log.Println("Opérateur non supporté dans deleteIDsFromCondition:", op)
		}
	}
}

// toInt64 convertit une valeur en int64 si possible
func toInt64(val any) (int64, error) {
	switch v := val.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("erreur conversion string en int64: %v", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("type non convertible en int64: %T", val)
	}
}

func parseToTime(val any) (time.Time, error) {
	switch v := val.(type) {
	case time.Time:
		return v.UTC(), nil
	case *time.Time:
		if v == nil {
			return time.Time{}, fmt.Errorf("nil *time.Time")
		}
		return v.UTC(), nil
	case int, int8, int16, int32, int64:
		n, _ := toInt64(v)
		if n > 1e12 {
			sec := n / 1000
			ms := n % 1000
			return time.Unix(sec, ms*1e6).UTC(), nil
		}
		return time.Unix(n, 0).UTC(), nil
	case uint, uint8, uint16, uint32, uint64:
		n, _ := toInt64(v)
		if n > 1e12 {
			sec := n / 1000
			ms := n % 1000
			return time.Unix(sec, ms*1e6).UTC(), nil
		}
		return time.Unix(n, 0).UTC(), nil
	case float32:
		sec := int64(v)
		nsec := int64((v - float32(sec)) * 1e9)
		return time.Unix(sec, nsec).UTC(), nil
	case float64:
		sec := int64(v)
		nsec := int64((v - float64(sec)) * 1e9)
		return time.Unix(sec, nsec).UTC(), nil
	case string:
		s := v

		// --- FIX: Nettoyage des guillemets ("...") ---
		// Si la chaine commence et finit par des guillemets, on les enlève
		if len(s) > 1 && s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
		}
		// ---------------------------------------------

		if s == "" {
			return time.Time{}, fmt.Errorf("empty time string")
		}
		// Try numeric first
		if num, err := strconv.ParseInt(s, 10, 64); err == nil {
			if num > 1e12 {
				sec := num / 1000
				ms := num % 1000
				return time.Unix(sec, ms*1e6).UTC(), nil
			}
			return time.Unix(num, 0).UTC(), nil
		}
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999999",
			"2006-01-02 15:04:05",
			"2006-01-02",
			time.RFC1123Z,
			time.RFC1123,
			time.RFC850,
			time.ANSIC,
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t.UTC(), nil
			}
		}
		return time.Time{}, fmt.Errorf("unsupported time format: %s", s)
	default:
		return time.Time{}, fmt.Errorf("unsupported time type: %T", val)
	}
}

// Helper pour décider si un champ doit être indexé en ZSET (Range) ou SET (Exact)
func shouldIndexAsZSet(field string, kind reflect.Kind) bool {
	// 1. EXCLUSION EXPLICITE DES IDs (C'est ça qui corrige ton bug !)
	// Même s'ils sont int64, on ne veut pas de perte de précision Float64
	if field == "user_id" || field == "profile_picture_id" || field == "conversation_id" || field == "id" {
		return false
	}

	// 2. Dates -> ZSET
	if field == "created_at" || field == "updated_at" || field == "joined_at" || field == "expires_at" || field == "birthdate" || field == "ban_expires_at" || field == "tolerance_time" {
		return true
	}

	// 3. Autres Nombres (Stats, Age, etc.) -> ZSET
	if kind == reflect.Int || kind == reflect.Int64 || kind == reflect.Float64 || kind == reflect.Int32 {
		return true
	}

	return false
}
