package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// Helper pour convertir les slices/maps en JSON string pour Redis
func prepareForRedis(m map[string]any) {
	for k, v := range m {
		if v == nil {
			continue
		}
		val := reflect.ValueOf(v)
		// Si c'est un tableau, une slice ou une map, on le transforme en JSON string
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Map || val.Kind() == reflect.Struct {
			b, err := json.Marshal(v)
			if err == nil {
				m[k] = string(b)
			}
		}
	}
}

// RedisCreateMedia insère le média dans le cache Redis
func RedisCreateMedia(m map[string]any) error {
	prepareForRedis(m)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return Media.Set(ctx, m)
}

// RedisCreateUser insère l'utilisateur dans le cache Redis avec indexation et LRU
func RedisCreateUser(u domain.UserRequest) error {
	// 1. Conversion Struct -> Map (comme pour Mongo)
	doc, err := pkg.ToMap(u)
	if err != nil {
		log.Printf("Erreur conversion map User pour Redis: %v", err)
		return err
	}

	prepareForRedis(doc)

	// 2. Contexte avec Timeout (sécurité pour ne pas bloquer l'API si Redis rame)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 3. Appel à ta collection Users du package cache
	// Grâce à ton code, cela va automatiquement :
	// - Créer le Hash "cache:users:ID"
	// - Créer les index (ex: ZSET pour created_at, SET pour username)
	// - Mettre à jour la LRU
	return Users.Set(ctx, doc)
}

// RedisCreateSession insère la session dans le cache Redis
func RedisCreateSession(s domain.SessionsRequest) error {
	doc, err := pkg.ToMap(s)
	if err != nil {
		log.Printf("Erreur conversion map Session pour Redis: %v", err)
		return err
	}

	prepareForRedis(doc)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Appel à ta collection Sessions du package cache
	return Sessions.Set(ctx, doc)
}

// RedisLoadUser charge un utilisateur depuis le cache Redis
func RedisLoadUser(ID int, Username string, Email string, Phone string) (domain.UserRequest, error) {
	var u domain.UserRequest

	// 1. Construction du filtre compatible avec ton ORM Redis
	// Rappel: Ton evalTree attend map[string]map[string]any
	// Ex: "username": { "$eq": "Marie" }
	filter := make(map[string]any)

	if ID != -1 {
		filter["id"] = map[string]any{"$eq": ID}
	}
	if Username != "" {
		filter["username"] = map[string]any{"$eq": Username}
	}
	if Email != "" {
		filter["email"] = map[string]any{"$eq": Email}
	}
	if Phone != "" {
		filter["phone"] = map[string]any{"$eq": Phone}
	}

	// Si aucun filtre n'est défini, on évite de tout charger (ou on retourne une erreur selon ta logique)
	if len(filter) == 0 {
		return u, fmt.Errorf("aucun critère de recherche fourni pour RedisLoadUser")
	}

	// 2. Création du contexte
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 3. Appel à Users.Get
	docs, err := Users.Get(ctx, filter)
	if err != nil {
		return u, err
	}

	// 4. Vérification si trouvé
	if len(docs) == 0 {
		return u, fmt.Errorf("utilisateur introuvable dans Redis")
	}

	if val, ok := docs[0]["badges"]; ok {
		if str, ok := val.(string); ok && str != "" {
			var badges []string
			if err := json.Unmarshal([]byte(str), &badges); err == nil {
				docs[0]["badges"] = badges
			}
		}
	}

	if err := pkg.ToStruct(docs[0], &u); err != nil {
		log.Printf("Erreur conversion Redis User vers Struct: %v", err)
		return u, err
	}

	return u, nil
}

// RedisLoadSession charge une session depuis le cache Redis
func RedisLoadSession(ID int, DeviceToken string) (domain.SessionsRequest, error) {
	var s domain.SessionsRequest

	// 1. Construction du filtre
	filter := make(map[string]any)

	if ID != -1 {
		filter["user_id"] = map[string]any{"$eq": ID}
	}
	if DeviceToken != "" {
		filter["device_token"] = map[string]any{"$eq": DeviceToken}
	}

	if len(filter) == 0 {
		return s, fmt.Errorf("aucun critère de recherche fourni pour RedisLoadSession")
	}

	// 2. Contexte
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 3. Appel à Sessions.Get
	docs, err := Sessions.Get(ctx, filter)
	if err != nil {
		return s, err
	}

	if len(docs) == 0 {
		return s, fmt.Errorf("session introuvable dans Redis")
	}

	if val, ok := docs[0]["device_info"]; ok {
		if str, ok := val.(string); ok && str != "" {
			var info map[string]any
			if err := json.Unmarshal([]byte(str), &info); err == nil {
				docs[0]["device_info"] = info
			}
		}
	}
	if val, ok := docs[0]["ip_history"]; ok {
		if str, ok := val.(string); ok && str != "" {
			var ips []string
			if err := json.Unmarshal([]byte(str), &ips); err == nil {
				docs[0]["ip_history"] = ips
			}
		}
	}

	if err := pkg.ToStruct(docs[0], &s); err != nil {
		log.Printf("Erreur conversion Redis Session vers Struct: %v", err)
		return s, err
	}

	return s, nil
}
