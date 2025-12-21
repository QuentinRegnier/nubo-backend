package redis

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/go-redis/redis/v8"
)

// ---------------- Initialisation ----------------
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
	Users = NewCollection("users", schemaUsers, redisgo.Rdb, GlobalStrategy)
	UserSettings = NewCollection("user_settings", schemaUserSettings, redisgo.Rdb, GlobalStrategy)
	Sessions = NewCollection("sessions", schemaSessions, redisgo.Rdb, GlobalStrategy)
	Relations = NewCollection("relations", schemaRelations, redisgo.Rdb, GlobalStrategy)
	Posts = NewCollection("posts", schemaPosts, redisgo.Rdb, GlobalStrategy)
	Comments = NewCollection("comments", schemaComments, redisgo.Rdb, GlobalStrategy)
	Likes = NewCollection("likes", schemaLikes, redisgo.Rdb, GlobalStrategy)
	Media = NewCollection("media", schemaMedia, redisgo.Rdb, GlobalStrategy)
	Conversations = NewCollection("conversations", schemaConversations, redisgo.Rdb, GlobalStrategy)
	Members = NewCollection("members", schemaMembers, redisgo.Rdb, GlobalStrategy)
	Messages = NewCollection("messages", schemaMessages, redisgo.Rdb, GlobalStrategy)

	log.Println("Structure Redis (caches) initialisée")
}

// ---------------- Collection et schéma ----------------

type Collection struct {
	Name       string                  // ex: "messages"
	Schema     map[string]reflect.Kind // ex: {"id": reflect.Int, "content": reflect.String}
	Redis      *redis.Client
	LRU        *LRUCache     // pour mettre à jour la LRU si cache
	Expiration time.Duration // TTL par défaut pour chaque élément, facultatif
}

// NewCollection crée une collection avec un schéma et LRU optionnel
func NewCollection(name string, schema map[string]reflect.Kind, rdb *redis.Client, lru *LRUCache) *Collection {
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

	return &Collection{
		Name:   name,
		Schema: schema,
		Redis:  rdb,
		LRU:    lru,
	}
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
		var isZSet bool
		var score float64

		// Est-ce une date ?
		if field == "created_at" || field == "updated_at" || field == "joined_at" || field == "expires_at" {
			if t, err := parseToTime(val); err == nil {
				isZSet = true
				score = float64(t.Unix())
			}
		} else {
			// Est-ce un nombre ?
			switch kind {
			case reflect.Int, reflect.Int64, reflect.Float64, reflect.Int32:
				if n, err := toInt64(val); err == nil {
					isZSet = true
					score = float64(n)
				}
			}
		}

		if isZSet {
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

	// Mise à jour LRU
	if c.LRU != nil {
		c.LRU.MarkUsed(c.Name, id)
	}

	return nil
}

// ---------------- Get ----------------

// Get retourne tous les éléments correspondant au filtre (MongoDB-like)
func (c *Collection) Get(ctx context.Context, filter map[string]any) ([]map[string]any, error) {
	// 1. Récupérer l’ensemble des IDs possibles via evalTree
	candidateSet, _, err := evalTree(ctx, c.Redis, c.Name, filter, "")
	if err != nil {
		return nil, err
	}

	// 2. Charger les objets correspondants
	results := []map[string]any{}
	for id := range candidateSet {
		objKey := "cache:" + c.Name + ":" + id
		data, err := c.Redis.HGetAll(ctx, objKey).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		obj := make(map[string]any)
		for k, v := range data {
			obj[k] = v
		}

		results = append(results, obj)

		if c.LRU != nil {
			c.LRU.MarkUsed(c.Name, id)
		}
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

			isZSet := false
			if field == "created_at" || field == "updated_at" || field == "joined_at" || field == "expires_at" {
				isZSet = true
			} else {
				switch kind {
				case reflect.Int, reflect.Int64, reflect.Float64, reflect.Int32:
					isZSet = true
				}
			}

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

		if c.LRU != nil {
			c.LRU.mu.Lock()
			delete(c.LRU.elements, c.Name+":"+id)
			c.LRU.mu.Unlock()
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
			var isZSet bool
			var newScore float64

			// Détection ZSET (Dates)
			if field == "created_at" || field == "updated_at" || field == "joined_at" || field == "expires_at" {
				if t, err := parseToTime(newVal); err == nil {
					isZSet = true
					newScore = float64(t.Unix())
				}
			} else {
				// Détection ZSET (Nombres)
				switch kind {
				case reflect.Int, reflect.Int64, reflect.Float64, reflect.Int32:
					if n, err := toInt64(newVal); err == nil {
						isZSet = true
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

		// 5. Mise à jour LRU
		if c.LRU != nil {
			c.LRU.MarkUsed(c.Name, id)
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
			ids, err := fetchIDsForCondition(ctx, rdb, collName, field, op, val)
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

// fetchIDsForCondition : récupère les IDs directement depuis Redis pour une condition simple.
func fetchIDsForCondition(ctx context.Context, rdb *redis.Client, collName, field, op string, val any) ([]string, error) {
	key := fmt.Sprintf("index:%s:%s", collName, field)

	switch op {
	case "$eq":
		if field == "id" {
			idStr := fmt.Sprintf("%v", val)
			objKey := fmt.Sprintf("cache:%s:%s", collName, idStr)
			exists, err := rdb.Exists(ctx, objKey).Result()
			if err != nil {
				return nil, err
			}
			if exists == 1 {
				return []string{idStr}, nil
			}
			return []string{}, nil
		}
		member := fmt.Sprintf("%v", val)
		ids, err := rdb.SMembers(ctx, key+":"+member).Result()
		if err != nil {
			return nil, err
		}
		return ids, nil

	case "$in":
		if field == "id" {
			vals, _ := val.([]any)
			var existing []string
			for _, v := range vals {
				idStr := fmt.Sprintf("%v", v)
				objKey := fmt.Sprintf("cache:%s:%s", collName, idStr)
				exists, err := rdb.Exists(ctx, objKey).Result()
				if err != nil {
					return nil, err
				}
				if exists == 1 {
					existing = append(existing, idStr)
				}
			}
			return existing, nil
		}
		vals, _ := val.([]any)
		var all []string
		for _, v := range vals {
			member := fmt.Sprintf("%v", v)
			ids, err := rdb.SMembers(ctx, key+":"+member).Result()
			if err != nil {
				return nil, err
			}
			all = append(all, ids...)
		}
		return all, nil

	case "$gt", "$gte", "$lt", "$lte":
		// 1. Convertir la valeur de référence en Score (float64)
		var score float64

		// Est-ce une date ? (basé sur tes noms de champs ou le type)
		if field == "created_at" || field == "updated_at" || field == "joined_at" || field == "expires_at" {
			t, err := parseToTime(val)
			if err != nil {
				return nil, fmt.Errorf("date invalide pour filtre %s: %v", field, err)
			}
			// On utilise le timestamp Unix comme score
			score = float64(t.Unix())
		} else {
			// C'est un nombre (int, float, etc.)
			valInt, err := toInt64(val) // Ta fonction utilitaire existante
			if err != nil {
				return nil, fmt.Errorf("valeur non numérique pour filtre %s: %v", field, err)
			}
			score = float64(valInt)
		}

		// 2. Préparer l'intervalle Redis (ZRangeBy)
		rBox := &redis.ZRangeBy{
			Min: "-inf",
			Max: "+inf",
		}

		scoreStr := fmt.Sprintf("%f", score) // Conversion propre en string pour Redis

		switch op {
		case "$gt":
			// "(" signifie exclusif dans la syntaxe Redis
			rBox.Min = "(" + scoreStr
		case "$gte":
			rBox.Min = scoreStr
		case "$lt":
			rBox.Max = "(" + scoreStr
		case "$lte":
			rBox.Max = scoreStr
		}

		// 3. Exécution atomique (plus de boucle for !)
		// Retourne directement tous les IDs dans l'intervalle
		ids, err := rdb.ZRangeByScore(ctx, key, rBox).Result()
		if err != nil {
			return nil, err
		}
		return ids, nil

	default:
		// Pas d'index → laisse le filtrage final (matchFilter) s'en occuper
		return []string{}, nil
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
		// Heuristic: treat >= 1e12 as milliseconds
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
