package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/tools"
	"github.com/go-redis/redis/v8"
)

// ---------------- Initialisation ----------------
// declarations globales
var (
	Users               *Collection
	UserSettings        *Collection
	Sessions            *Collection
	Relations           *Collection
	Posts               *Collection
	Comments            *Collection
	Likes               *Collection
	Media               *Collection
	ConversationsMeta   *Collection
	ConversationMembers *Collection
	Messages            *Collection
)

// InitCacheDatabase initialise la structure logique de Redis pour les caches
func InitCacheDatabase() {
	// Initialiser les collections

	schemaUsers := tools.UsersSchema
	schemaUserSettings := tools.UserSettingsSchema
	schemaSessions := tools.SessionsSchema
	schemaRelations := tools.RelationsSchema
	schemaPosts := tools.PostsSchema
	schemaComments := tools.CommentsSchema
	schemaLikes := tools.LikesSchema
	schemaMedia := tools.MediaSchema
	schemaConversationsMeta := tools.ConversationsMetaSchema
	schemaConversationMembers := tools.ConversationMembersSchema
	schemaMessages := tools.MessagesSchema

	// variables globales
	Users = NewCollection("users", schemaUsers, Rdb, GlobalStrategy)
	UserSettings = NewCollection("user_settings", schemaUserSettings, Rdb, GlobalStrategy)
	Sessions = NewCollection("sessions", schemaSessions, Rdb, GlobalStrategy)
	Relations = NewCollection("relations", schemaRelations, Rdb, GlobalStrategy)
	Posts = NewCollection("posts", schemaPosts, Rdb, GlobalStrategy)
	Comments = NewCollection("comments", schemaComments, Rdb, GlobalStrategy)
	Likes = NewCollection("likes", schemaLikes, Rdb, GlobalStrategy)
	Media = NewCollection("media", schemaMedia, Rdb, GlobalStrategy)
	ConversationsMeta = NewCollection("conversations_meta", schemaConversationsMeta, Rdb, GlobalStrategy)
	ConversationMembers = NewCollection("conversation_members", schemaConversationMembers, Rdb, GlobalStrategy)
	Messages = NewCollection("messages", schemaMessages, Rdb, GlobalStrategy)

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
		if reflect.TypeOf(val).Kind() != kind {
			return fmt.Errorf("champ %s doit être de type %s", field, kind.String())
		}
	}
	return nil
}

// ---------------- Set ----------------

// Set ajoute un élément dans la collection
func (c *Collection) Set(ctx context.Context, obj map[string]any) error {
	if err := c.validate(obj); err != nil {
		log.Println("Validation échouée:", err)
		return err
	}

	id := fmt.Sprintf("%v", obj["id"])
	objKey := "cache:" + c.Name + ":" + id

	// Sauvegarde complète dans Redis Hash
	if err := c.Redis.HMSet(ctx, objKey, obj).Err(); err != nil {
		return err
	}

	// Mettre à jour les indexs
	for field := range c.Schema {
		if field == "id" {
			continue
		}
		if val, ok := obj[field]; ok {
			valStr := fmt.Sprintf("%v", val)
			idxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, valStr)
			if err := c.Redis.SAdd(ctx, idxKey, id).Err(); err != nil {
				log.Printf("Erreur mise à jour index %s: %v", idxKey, err)
			}
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

// Delete supprime les éléments correspondant au filtre et nettoie les index vides
func (c *Collection) Delete(ctx context.Context, filter map[string]any) error {

	// Récupérer les objets via Get (filtrage complet)
	objs, err := c.Get(ctx, filter)
	if err != nil {
		return err
	}

	pipe := c.Redis.TxPipeline()
	// Stocker les paires idxKey -> id pour vérifier après
	type idxCheck struct {
		idxKey string
	}
	var checks []idxCheck

	for _, obj := range objs {
		id := fmt.Sprintf("%v", obj["id"])
		objKey := "cache:" + c.Name + ":" + id

		// Supprimer le hash principal
		pipe.Del(ctx, objKey)

		// Supprimer l’ID de tous les indexs
		for field := range c.Schema {
			if field == "id" {
				continue
			}
			if val, ok := obj[field]; ok {
				valStr := fmt.Sprintf("%v", val)
				idxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, valStr)
				pipe.SRem(ctx, idxKey, id)
				checks = append(checks, idxCheck{idxKey: idxKey})
			}
		}

		// Nettoyer la LRU
		if c.LRU != nil {
			c.LRU.mu.Lock()
			delete(c.LRU.elements, c.Name+":"+id)
			c.LRU.mu.Unlock()
		}
	}

	// Exécuter le pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("Erreur exécution pipeline delete: %v", err)
		return err
	}

	// Vérifier et supprimer les index vides
	for _, chk := range checks {
		count, err := c.Redis.SCard(ctx, chk.idxKey).Result()
		if err != nil {
			log.Printf("Erreur lecture index %s: %v", chk.idxKey, err)
			continue
		}
		if count == 0 {
			if err := c.Redis.Del(ctx, chk.idxKey).Err(); err != nil {
				log.Printf("Erreur suppression index vide %s: %v", chk.idxKey, err)
			} else {
				log.Printf("Index vide supprimé: %s", chk.idxKey)
			}
		}
	}

	return nil
}

// ---------------- Update ----------------

// Update met à jour les éléments correspondant au filtre avec les nouvelles valeurs fournies dans update
func (c *Collection) Update(ctx context.Context, filter map[string]interface{}, update map[string]interface{}) error {

	// Récupérer les objets correspondant au filtre
	objs, err := c.Get(ctx, filter)
	if err != nil {
		return err
	}

	pipe := c.Redis.TxPipeline()

	for _, obj := range objs {
		id := fmt.Sprintf("%v", obj["id"])
		objKey := "cache:" + c.Name + ":" + id

		// Mettre à jour l'objet avec les nouvelles valeurs
		for field, val := range update {
			obj[field] = val
		}

		// Sérialiser et stocker dans Redis
		data, _ := json.Marshal(obj)
		pipe.Set(ctx, objKey, data, 0)

		// Mettre à jour la LRU si nécessaire
		if c.LRU != nil {
			c.LRU.MarkUsed(c.Name, id)
		}

		// Mettre à jour les index
		for field := range c.Schema {
			if field == "id" {
				continue
			}
			// Supprimer l'ancien index si la valeur a changé
			if oldVal, ok := obj[field]; ok {
				oldValStr := fmt.Sprintf("%v", oldVal)
				idxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, oldValStr)
				pipe.SAdd(ctx, idxKey, id) // ajouter au nouvel index (SRem est déjà géré dans Delete si on le souhaite)
			}
		}
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Printf("Erreur execution pipeline Modify: %v", err)
		return err
	}

	return nil
}

// ---------------- Filtrage ----------------

func evalTree(ctx context.Context, redis *redis.Client, collName string, filter map[string]any, type_before string) (map[string]struct{}, []map[string]any, error) {
	// Cas 1 : opérateurs logiques
	if orOps, ok := filter["$or"]; ok {
		arr, _ := orOps.([]any)
		unionSet := make(map[string]struct{})
		for _, sub := range arr {
			subFilter, _ := sub.(map[string]any)
			res, _, err := evalTree(ctx, redis, collName, subFilter, "$or")
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
			res, del, err := evalTree(ctx, redis, collName, subFilter, "$and")
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
			ids, err := fetchIDsForCondition(ctx, redis, collName, field, op, val)
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
func fetchIDsForCondition(ctx context.Context, redis *redis.Client, collName, field, op string, val any) ([]string, error) {
	key := fmt.Sprintf("index:%s:%s", collName, field)

	switch op {
	case "$eq":
		if field == "id" {
			idStr := fmt.Sprintf("%v", val)
			objKey := fmt.Sprintf("cache:%s:%s", collName, idStr)
			exists, err := redis.Exists(ctx, objKey).Result()
			if err != nil {
				return nil, err
			}
			if exists == 1 {
				return []string{idStr}, nil
			}
			return []string{}, nil
		}
		member := fmt.Sprintf("%v", val)
		ids, err := redis.SMembers(ctx, key+":"+member).Result()
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
				exists, err := redis.Exists(ctx, objKey).Result()
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
			ids, err := redis.SMembers(ctx, key+":"+member).Result()
			if err != nil {
				return nil, err
			}
			all = append(all, ids...)
		}
		return all, nil

	case "$gt", "$gte", "$lt", "$lte":
		if field == "id" {
			switch op {
			case "$gt":
				f, err := toInt64(val)
				if err != nil {
					return []string{}, nil
				}
				start := int64(f)
				var ids []string
				for i := start + 1; ; i++ {
					objKey := fmt.Sprintf("cache:%s:%d", collName, i)
					exists, err := redis.Exists(ctx, objKey).Result()
					if err != nil {
						return nil, err
					}
					if exists == 0 {
						break
					}
					ids = append(ids, strconv.FormatInt(i, 10))
				}
				return ids, nil
			case "$gte":
				f, err := toInt64(val)
				if err != nil {
					return []string{}, nil
				}
				start := int64(f)
				var ids []string
				for i := start; ; i++ {
					objKey := fmt.Sprintf("cache:%s:%d", collName, i)
					exists, err := redis.Exists(ctx, objKey).Result()
					if err != nil {
						return nil, err
					}
					if exists == 0 {
						break
					}
					ids = append(ids, strconv.FormatInt(i, 10))
				}
				return ids, nil
			case "$lt":
				f, err := toInt64(val)
				if err != nil {
					return []string{}, nil
				}
				end := int64(f)
				var ids []string
				for i := int64(0); i < end; i++ {
					objKey := fmt.Sprintf("cache:%s:%d", collName, i)
					exists, err := redis.Exists(ctx, objKey).Result()
					if err != nil {
						return nil, err
					}
					if exists == 0 {
						break
					}
					ids = append(ids, strconv.FormatInt(i, 10))
				}
				return ids, nil
			case "$lte":
				f, err := toInt64(val)
				if err != nil {
					return []string{}, nil
				}
				end := int64(f)
				var ids []string
				for i := int64(0); i <= end; i++ {
					objKey := fmt.Sprintf("cache:%s:%d", collName, i)
					exists, err := redis.Exists(ctx, objKey).Result()
					if err != nil {
						return nil, err
					}
					if exists == 0 {
						break
					}
					ids = append(ids, strconv.FormatInt(i, 10))
				}
				return ids, nil

			default:
				return []string{}, nil
			}
		}
		if field == "created_at" || field == "updated_at" || field == "joined_at" || field == "expires_at" {
			switch op {
			case "$gt":
				tRef, err := parseToTime(val)
				if err != nil {
					return []string{}, nil
				}
				ids := []string{}
				start := tRef.Add(24 * time.Hour)
				for t := start; t.Before(time.Now().UTC()); t = t.Add(24 * time.Hour) {
					member := fmt.Sprintf("%v", t)
					memberIDs, err := redis.SMembers(ctx, key+":"+member).Result()
					if err != nil {
						return nil, err
					}
					ids = append(ids, memberIDs...)
				}
				return ids, nil
			case "$gte":
				tRef, err := parseToTime(val)
				if err != nil {
					return []string{}, nil
				}
				ids := []string{}
				start := tRef
				for t := start; t.Before(time.Now().UTC()); t = t.Add(24 * time.Hour) {
					member := fmt.Sprintf("%v", t)
					memberIDs, err := redis.SMembers(ctx, key+":"+member).Result()
					if err != nil {
						return nil, err
					}
					ids = append(ids, memberIDs...)
				}
				return ids, nil
			case "$lt":
				tRef, err := parseToTime(val)
				if err != nil {
					return []string{}, nil
				}
				ids := []string{}
				start := time.Date(2025, time.September, 14, 0, 0, 0, 0, time.UTC)
				for t := start; t.Before(tRef); t = t.Add(24 * time.Hour) {
					member := fmt.Sprintf("%v", t)
					memberIDs, err := redis.SMembers(ctx, key+":"+member).Result()
					if err != nil {
						return nil, err
					}
					ids = append(ids, memberIDs...)
				}
				return ids, nil
			case "$lte":
				tRef, err := parseToTime(val)
				if err != nil {
					return []string{}, nil
				}
				ids := []string{}
				start := time.Date(2025, time.September, 14, 0, 0, 0, 0, time.UTC)
				for t := start; !t.After(tRef); t = t.Add(24 * time.Hour) {
					member := fmt.Sprintf("%v", t)
					memberIDs, err := redis.SMembers(ctx, key+":"+member).Result()
					if err != nil {
						return nil, err
					}
					ids = append(ids, memberIDs...)
				}
				return ids, nil
			case "$in":

			default:
				return []string{}, nil
			}
		}
		// Pas de condition sur les dates
		return []string{}, nil

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
