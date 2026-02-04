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
func PrepareForRedis(m map[string]any) {
	for k, v := range m {
		if v == nil {
			continue
		}
		val := reflect.ValueOf(v)
		// Si c'est un tableau, une slice ou une map, on le transforme en JSON string
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Map || val.Kind() == reflect.Struct {
			// Petit fix de sécurité : on ignore les Time qui sont des structs mais gérés nativement par ton manager
			if _, isTime := v.(time.Time); isTime {
				continue
			}
			b, err := json.Marshal(v)
			if err == nil {
				m[k] = string(b)
			}
		}
	}
}

// RedisCreateMedia insère le média dans le cache Redis
func RedisCreateMedia(m domain.MediaRequest) error {
	doc, err := pkg.ToMap(m)
	if err != nil {
		log.Printf("Erreur conversion map Media pour Redis: %v", err)
		return err
	}

	PrepareForRedis(doc)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return Media.Set(ctx, doc)
}

// RedisCreateUser insère l'utilisateur dans le cache Redis avec indexation et LRU
func RedisCreateUser(u domain.UserRequest) error {
	// --- CORRECTION : MAPPING MANUEL ---
	// On évite pkg.ToMap pour conserver les int64 intacts
	doc := map[string]any{
		"id":                 u.ID,
		"username":           u.Username,
		"email":              u.Email,
		"phone":              u.Phone,
		"password_hash":      u.PasswordHash,
		"first_name":         u.FirstName,
		"last_name":          u.LastName,
		"bio":                u.Bio,
		"profile_picture_id": u.ProfilePictureID, // int64 conservé !
		"birthdate":          u.Birthdate,
		"created_at":         u.CreatedAt,
		"updated_at":         u.UpdatedAt,
		"desactivated":       u.Desactivated,
		"banned":             u.Banned,
		"ban_reason":         u.BanReason,
		"ban_expires_at":     u.BanExpiresAt,
		"email_verified":     u.EmailVerified,
		"phone_verified":     u.PhoneVerified,
		"sex":                u.Sex,
		"grade":              u.Grade,
		"school":             u.School,
		"work":               u.Work,
		"location":           u.Location,
		"badges":             u.Badges, // Sera converti en string JSON par prepareForRedis
	}

	// Transforme les tableaux/maps (badges, etc.) en JSON string
	PrepareForRedis(doc)

	// 2. Contexte avec Timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 3. Appel à Users.Set
	return Users.Set(ctx, doc)
}

// RedisCreateSession insère la session dans le cache Redis
func RedisCreateSession(s domain.SessionsRequest) error {
	fmt.Printf("🐞 DEBUG GO STRUCT: ID=%d, UserID=%d\n", s.ID, s.UserID)
	// --- CORRECTION : MAPPING MANUEL (C'est ici que ton bug était !) ---
	doc := map[string]any{
		"id":             s.ID,     // int64 (Snowflake)
		"user_id":        s.UserID, // int64 (Snowflake) - RESTERA INT64 !
		"device_token":   s.DeviceToken,
		"master_token":   s.MasterToken,
		"current_secret": s.CurrentSecret,
		"last_secret":    s.LastSecret,
		"last_jwt":       s.LastJWT,
		"created_at":     s.CreatedAt,
		"expires_at":     s.ExpiresAt,
		"tolerance_time": s.ToleranceTime,
		"device_info":    s.DeviceInfo, // Sera converti par prepareForRedis
		"ip_history":     s.IPHistory,  // Sera converti par prepareForRedis
	}

	fmt.Printf("🐞 DEBUG MAP REDIS: ID=%v, UserID=%v\n", doc["id"], doc["user_id"])

	// Transforme device_info et ip_history en JSON string
	PrepareForRedis(doc)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Appel à Sessions.Set
	return Sessions.Set(ctx, doc)
}

// RedisLoadUser charge un utilisateur depuis le cache Redis
func RedisLoadUser(ID int64, Username string, Email string, Phone string) (domain.UserRequest, error) {
	fmt.Println("RedisLoadUser called with:", ID, Username, Email, Phone)
	var u domain.UserRequest

	// 1. Construction du filtre compatible avec ton ORM Redis
	// Rappel: Ton evalTree attend map[string]map[string]any
	// Ex: "username": { "$eq": "Marie" }
	filter := make(map[string]any)

	if ID != -1 && ID != 0 {
		filter["id"] = map[string]any{"$eq": ID}
	} else if Email != "" {
		filter["email"] = map[string]any{"$eq": Email}
	} else if Username != "" {
		filter["username"] = map[string]any{"$eq": Username}
	} else if Phone != "" {
		filter["phone"] = map[string]any{"$eq": Phone}
	} else {
		return domain.UserRequest{}, fmt.Errorf("aucun critère de recherche")
	}

	fmt.Println("RedisLoadUser filter:", filter)

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
func RedisLoadSession(userID int64, DeviceToken string, MasterToken string, CurrentSecret string) (domain.SessionsRequest, error) {
	fmt.Println("RedisLoadSession called with:", userID, DeviceToken, MasterToken, CurrentSecret)
	var s domain.SessionsRequest

	// 1. Construction du filtre
	filter := make(map[string]any)

	if userID != -1 {
		filter["user_id"] = map[string]any{"$eq": userID}
	}
	if DeviceToken != "" {
		filter["device_token"] = map[string]any{"$eq": DeviceToken}
	}
	if MasterToken != "" {
		filter["master_token"] = map[string]any{"$eq": MasterToken}
	}
	if CurrentSecret != "" {
		filter["current_secret"] = map[string]any{"$eq": CurrentSecret}
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
func RedisUpdateSession(s domain.SessionsRequest) error {
	// --- CORRECTION : MAPPING MANUEL ---
	// Pour l'update, on doit s'assurer de passer les bons types pour ce qu'on met à jour
	doc := map[string]any{
		"user_id":        s.UserID,
		"device_token":   s.DeviceToken,
		"master_token":   s.MasterToken,
		"current_secret": s.CurrentSecret,
		"last_secret":    s.LastSecret,
		"last_jwt":       s.LastJWT,
		"created_at":     s.CreatedAt,
		"expires_at":     s.ExpiresAt,
		"tolerance_time": s.ToleranceTime,
		"device_info":    s.DeviceInfo,
		"ip_history":     s.IPHistory,
	}

	// Si l'ID est dans la struct mais qu'on ne veut pas l'update (c'est la clé primaire), on l'enlève de la map d'update
	// (Dans ton code précédent tu le supprimais après ToMap, ici on ne l'a juste pas mis dans doc)

	PrepareForRedis(doc)

	// 4. Construction du filtre
	filter := make(map[string]any)

	// On utilise l'ID pour cibler l'objet à mettre à jour
	if s.ID != 0 {
		filter["id"] = map[string]any{"$eq": s.ID}
	} else if s.UserID != 0 && s.DeviceToken != "" {
		// Fallback si on n'a pas l'ID de session direct
		filter["user_id"] = map[string]any{"$eq": s.UserID}
		filter["device_token"] = map[string]any{"$eq": s.DeviceToken}
	} else {
		return fmt.Errorf("RedisUpdateSession: impossible de cibler la session (manque ID ou UserID+DeviceToken)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return Sessions.Update(ctx, filter, doc)
}
func RedisCreatePost(s domain.PostRequest) error {
	// --- CORRECTION : MAPPING MANUEL ---
	doc := map[string]any{
		"id":          s.ID,
		"user_id":     s.UserID,
		"content":     s.Content,
		"hashtags":    s.Hashtags,
		"identifiers": s.Identifiers,
		"media_ids":   s.MediaIDs,
		"visibility":  s.Visibility,
		"location":    s.Location,
		"created_at":  s.CreatedAt,
		"updated_at":  s.UpdatedAt,
	}

	PrepareForRedis(doc)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return Posts.Set(ctx, doc)
}
