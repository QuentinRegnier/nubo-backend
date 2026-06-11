package service

import (
	"context"
	"fmt"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ─────────────────────────────────────────────────────────────────────────────
// MÉMOIRE DU FEED (Cuckoo Filter Distribué via RedisBloom)
// ─────────────────────────────────────────────────────────────────────────────

// HasSeen vérifie dans le Cuckoo Filter Redis si l'utilisateur a déjà vu ce post_service.
// Complexité : O(1)
func HasSeen(ctx context.Context, userID int64, postID int64) bool {
	// L'appel utilise RedisBloom (module Redis).
	// Isolation stricte de la commande d'infrastructure probabiliste RedisBloom
	res, err := redis.CuckooSeen.CFExists(ctx, userID, postID)
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
	// CF.ADD crée automatiquement le filtre s'il n'existe pas via notre abstraction L1.
	err := redis.CuckooSeen.CFAdd(ctx, userID, postID)
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

	// Extension automatique de l'index glissant en mémoire volatile
	_ = redis.CuckooSeen.RefreshTTL(ctx, userID)
}

// ResetCuckooFilter purge l'intégralité du filtre RedisBloom de l'utilisateur de la RAM (L1).
// Indispensable pour éviter la saturation sémantique lors des rafraîchissements destructifs (/force).
func ResetCuckooFilter(ctx context.Context, userID int64) {
	_ = redis.CuckooSeen.DeleteObject(ctx, userID)
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
