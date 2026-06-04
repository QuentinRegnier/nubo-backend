package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
)

// ─────────────────────────────────────────────────────────────────────────────
// MÉMOIRE DU FEED (Cuckoo Filter Distribué via RedisBloom)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// Clé Redis pour le Cuckoo Filter de l'utilisateur.
	RedisKeyCuckooSeen = "cuckoo:seen:%d"
	// Durée de mémorisation avant expiration totale (hygiène de la RAM).
	CuckooSeenTTL = 7 * 24 * time.Hour
)

// HasSeen vérifie dans le Cuckoo Filter Redis si l'utilisateur a déjà vu ce post_service.
// Complexité : O(1)
func HasSeen(ctx context.Context, userID int64, postID int64) bool {
	key := fmt.Sprintf(RedisKeyCuckooSeen, userID)

	// L'appel utilise RedisBloom (module Redis).
	// On utilise Do() pour exécuter la commande brute "CF.EXISTS"
	// si elle n'est pas nativement wrappée par le client.
	res, err := redisgo.Rdb.Do(ctx, "CF.EXISTS", key, postID).Bool()
	if err != nil {
		// ---------------------------------------------------------
		// FALLBACK TRANSPARENT
		// ---------------------------------------------------------
		// Si le filtre n'existe pas, que le module RedisBloom n'est pas chargé,
		// ou qu'il y a un timeout réseau : on retourne 'false'.
		// Conséquence : Le post_service est accepté dans le panier. L'utilisateur
		// risque de voir un doublon, mais l'application ne crashe pas.
		return false
	}

	return res
}

// MarkAsSeen insère le post_service dans le Cuckoo Filter Redis de l'utilisateur.
// Complexité : O(1)
func MarkAsSeen(ctx context.Context, userID int64, postID int64) {
	key := fmt.Sprintf(RedisKeyCuckooSeen, userID)

	// CF.ADD crée automatiquement le filtre s'il n'existe pas.
	_, err := redisgo.Rdb.Do(ctx, "CF.ADD", key, postID).Result()
	if err != nil {
		// ---------------------------------------------------------
		// FALLBACK & ALERTE SILENCIEUSE
		// ---------------------------------------------------------
		// Si le filtre est plein ("Cuckoo filter is full"), on ne bloque pas.
		// On log l'erreur pour la supervision (Kibana/Grafana) afin d'indiquer
		// qu'il faudra utiliser CF.RESERVE avec une plus grande capacité à l'avenir.
		log.Printf("⚠️ [MarkAsSeen] Impossible d'ajouter au CF (user: %d, post_service: %d) : %v", userID, postID, err)
		return
	}

	// À chaque ajout, on repousse le TTL du filtre pour éviter
	// qu'il ne reste indéfiniment en RAM si l'utilisateur devient inactif.
	redisgo.Rdb.Expire(ctx, key, CuckooSeenTTL)
}

// ─────────────────────────────────────────────────────────────────────────────
// VERIFICATION GLOBALE D'UNICITÉ (Méthode Existante)
// ─────────────────────────────────────────────────────────────────────────────

// IsUnique vérifie l'unicité d'une valeur (0 = existe déjà, 1 = unique)
func IsUnique(collection *mongo.MongoCollection, field string, value any) int {

	valStr := fmt.Sprintf("%v", value)
	key := field + ":" + valStr

	// ---------------------------------------------------------
	// 0. CUCKOO FILTER (RAM Layer - O(1))
	// ---------------------------------------------------------
	// Premier check ultra-rapide.
	// Si le Cuckoo ne le trouve pas, c'est CERTAIN qu'il n'existe pas.
	// On évite Redis, Mongo et Postgres.
	if cuckoo.GlobalCuckoo != nil {
		if !cuckoo.GlobalCuckoo.Lookup([]byte(key)) {
			return 1 // Unique (certitude 100%)
		}
	}
	// Si trouvé ici -> C'est PEUT-ÊTRE un doublon (ou faux positif).
	// On continue les vérifications pour confirmer.

	// ---------------------------------------------------------
	// 1. REDIS (Cache Layer)
	// ---------------------------------------------------------
	// Construction de la clé : "table:field:value"
	redisKey := fmt.Sprintf("%s:%s", collection.Name, key)

	// Utilisation d'un contexte vide (à adapter si tu passes le context.Context dans IsUnique à l'avenir)
	ctx := context.Background()

	// On vérifie l'existence dans le SPEED Cache
	// (Assure-toi d'utiliser la méthode Exists ou Get adaptée à ton package redis/calls.go)
	exists, err := redis.Exists(ctx, redisKey)
	if err != nil {
		log.Printf("Erreur IsUnique (Redis) : %v", err)
		// En cas d'erreur Redis, on ne bloque pas, on laisse Mongo/Postgres faire le travail
	} else if exists {
		return 0 // Existe déjà (Hit confirmé)
	}

	// ---------------------------------------------------------
	// 2. MONGODB
	// ---------------------------------------------------------
	filter := map[string]any{
		field: value,
	}
	projection := map[string]any{
		"id": 1,
	}
	// Get est dans generic.go dans le package mongo
	results, err := collection.Get(filter, projection)
	if err != nil {
		log.Printf("Erreur IsUnique (Mongo Get) : %v", err)
		return 0 // Sécurité
	}

	if len(results) > 0 {
		return 0 // Existe déjà
	}

	// ---------------------------------------------------------
	// 3. POSTGRESQL
	// ---------------------------------------------------------
	query := fmt.Sprintf("SELECT count(1) FROM %s WHERE %s = $1", collection.Name, field)
	var countSQL int
	err = postgres.PostgresDB.QueryRow(query, value).Scan(&countSQL)
	if err != nil {
		log.Printf("Erreur IsUnique (Postgres) : %v", err)
	}

	if countSQL > 0 {
		return 0 // Existe déjà
	}

	// ---------------------------------------------------------
	// Résultat final : La valeur est unique partout
	// ---------------------------------------------------------
	return 1
}
