package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

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

	schemaUsers := UsersSchema
	schemaUserSettings := UserSettingsSchema
	schemaSessions := SessionsSchema
	schemaRelations := RelationsSchema
	schemaPosts := PostsSchema
	schemaComments := CommentsSchema
	schemaLikes := LikesSchema
	schemaMedia := MediaSchema
	schemaConversationsMeta := ConversationsMetaSchema
	schemaConversationMembers := ConversationMembersSchema
	schemaMessages := MessagesSchema

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
func (c *Collection) Set(obj map[string]any) error {
	if err := c.validate(obj); err != nil {
		log.Println("Validation échouée:", err)
		return err
	}

	id := fmt.Sprintf("%v", obj["id"])
	objKey := "cache:" + c.Name + ":" + id

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

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
func (c *Collection) Get(filter map[string]any) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var candidateIDs []string

	// 🔹 Étape 1 : Réduire l’espace de recherche avec les index Redis
	indexKeys := []string{}
	for field, condition := range filter {
		subCond, ok := condition.(map[string]any)
		if !ok {
			// équivalent $eq direct
			valStr := fmt.Sprintf("%v", condition)
			idxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, valStr)
			indexKeys = append(indexKeys, idxKey)
			continue
		}

		for op, val := range subCond {
			switch op {
			case "$eq":
				valStr := fmt.Sprintf("%v", val)
				idxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, valStr)
				indexKeys = append(indexKeys, idxKey)

			case "$in":
				arr, ok := val.([]any)
				if ok {
					orKeys := []string{}
					for _, a := range arr {
						valStr := fmt.Sprintf("%v", a)
						idxKey := fmt.Sprintf("idx:%s:%s:%s", c.Name, field, valStr)
						orKeys = append(orKeys, idxKey)
					}
					// on mettra ça en union après
					if len(orKeys) > 0 {
						members, err := c.Redis.SUnion(ctx, orKeys...).Result()
						if err == nil {
							candidateIDs = append(candidateIDs, members...)
						}
					}
				}
			}
		}
	}

	// Si on a plusieurs indexKeys (issus de $eq), on fait une intersection
	if len(indexKeys) == 1 {
		ids, err := c.Redis.SMembers(ctx, indexKeys[0]).Result()
		if err == nil {
			candidateIDs = append(candidateIDs, ids...)
		}
	} else if len(indexKeys) > 1 {
		ids, err := c.Redis.SInter(ctx, indexKeys...).Result()
		if err == nil {
			candidateIDs = append(candidateIDs, ids...)
		}
	}

	// Si aucun index n’a filtré → on doit scanner tout
	if len(candidateIDs) == 0 {
		pattern := fmt.Sprintf("cache:%s:*", c.Name)
		keys, scanErr := c.Redis.Keys(ctx, pattern).Result()
		if scanErr != nil {
			return nil, scanErr
		}
		for _, k := range keys {
			parts := strings.Split(k, ":")
			candidateIDs = append(candidateIDs, parts[len(parts)-1])
		}
	}

	// 🔹 Étape 2 : Charger les objets et appliquer matchFilter
	results := []map[string]any{}
	for _, id := range candidateIDs {
		objKey := "cache:" + c.Name + ":" + id
		data, err := c.Redis.HGetAll(ctx, objKey).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		obj := make(map[string]any)
		for k, v := range data {
			obj[k] = v
		}

		// Vérification complète via matchFilter
		match, err := matchFilter(obj, filter)
		if err != nil {
			continue
		}
		if match {
			results = append(results, obj)
			if c.LRU != nil {
				c.LRU.MarkUsed(c.Name, id)
			}
		}
	}

	return results, nil
}

// ----------- Delete ----------------

// Delete supprime les éléments correspondant au filtre et nettoie les index vides
func (c *Collection) Delete(filter map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Récupérer les objets via Get (filtrage complet)
	objs, err := c.Get(filter)
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

// ---------------- Modify ----------------

// Modify met à jour les éléments correspondant au filtre avec les nouvelles valeurs fournies dans update
func (c *Collection) Modify(filter map[string]interface{}, update map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Récupérer les objets correspondant au filtre
	objs, err := c.Get(filter)
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

// matchFilter applique le filtre type MongoDB sur un objet
func matchFilter(obj map[string]any, filter map[string]any) (bool, error) {
	for k, v := range filter {
		if strings.HasPrefix(k, "$") {
			switch k {
			case "$and":
				arr, ok := v.([]any)
				if !ok {
					return false, fmt.Errorf("$and doit être un tableau")
				}
				for _, cond := range arr {
					subFilter, ok := cond.(map[string]any)
					if !ok {
						return false, fmt.Errorf("condition $and invalide")
					}
					match, err := matchFilter(obj, subFilter)
					if err != nil || !match {
						return false, err
					}
				}
				return true, nil
			case "$or":
				arr, ok := v.([]any)
				if !ok {
					return false, fmt.Errorf("$or doit être un tableau")
				}
				for _, cond := range arr {
					subFilter, ok := cond.(map[string]any)
					if !ok {
						return false, fmt.Errorf("condition $or invalide")
					}
					match, err := matchFilter(obj, subFilter)
					if err == nil && match {
						return true, nil
					}
				}
				return false, nil
			case "$not":
				subFilter, ok := v.(map[string]any)
				if !ok {
					return false, fmt.Errorf("$not doit être un objet")
				}
				match, err := matchFilter(obj, subFilter)
				return !match, err
			case "$nor":
				arr, ok := v.([]any)
				if !ok {
					return false, fmt.Errorf("$nor doit être un tableau")
				}
				for _, cond := range arr {
					subFilter, ok := cond.(map[string]any)
					if !ok {
						return false, fmt.Errorf("condition $nor invalide")
					}
					match, err := matchFilter(obj, subFilter)
					if err == nil && match {
						return false, nil
					}
				}
				return true, nil
			}
		} else {
			// opérateurs de comparaison
			subCond, ok := v.(map[string]any)
			if !ok {
				// équivalent $eq par défaut
				if obj[k] != v {
					return false, nil
				}
				continue
			}
			for op, val := range subCond {
				switch op {
				case "$eq":
					if obj[k] != val {
						return false, nil
					}
				case "$ne":
					if obj[k] == val {
						return false, nil
					}
				case "$gt":
					if !compareNumbers(obj[k], val, ">") {
						return false, nil
					}
				case "$gte":
					if !compareNumbers(obj[k], val, ">=") {
						return false, nil
					}
				case "$lt":
					if !compareNumbers(obj[k], val, "<") {
						return false, nil
					}
				case "$lte":
					if !compareNumbers(obj[k], val, "<=") {
						return false, nil
					}
				case "$in":
					arr, ok := val.([]any)
					if !ok {
						return false, nil
					}
					found := false
					for _, a := range arr {
						if a == obj[k] {
							found = true
							break
						}
					}
					if !found {
						return false, nil
					}
				case "$nin":
					arr, ok := val.([]any)
					if !ok {
						return false, nil
					}
					for _, a := range arr {
						if a == obj[k] {
							return false, nil
						}
					}
				}
			}
		}
	}
	return true, nil
}

func compareNumbers(a, b any, op string) bool {
	af, aok := toFloat64(a)
	bf, bok := toFloat64(b)
	if !aok || !bok {
		return false
	}
	switch op {
	case ">":
		return af > bf
	case ">=":
		return af >= bf
	case "<":
		return af < bf
	case "<=":
		return af <= bf
	}
	return false
}

func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	default:
		return 0, false
	}
}
