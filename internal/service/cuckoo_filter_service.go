package service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
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
		logger.Log.Warn().
			Err(err).
			Int64("user_id", userID).
			Int64("post_id", postID).
			Msg("Impossible d'ajouter au Cuckoo Filter")
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
// IsUnique exécute la cascade de vérification d'unicité en 5 étapes.
// Retourne 1 si la donnée est absolument unique, 0 si elle existe déjà.
// IsUnique est universel. Si l'entité n'est pas supportée, elle loggue une erreur et refuse l'action par sécurité.
func IsUnique(ctx context.Context, entity redis.EntityType, field string, value string) int {
	// ÉTAPE 1 & 2 : Cuckoo Filter (Moteur RAM universel)
	mightExist, hasFilter := cuckoo.MightExist(entity, field, value)
	if hasFilter && !mightExist {
		return 1 // Sûr à 100% que c'est unique. Zéro appel BDD !
	}

	// ÉTAPE 4 : MONGODB (Warm Storage L2)
	existsMongo, errMongo := mongo.MongoCheckUnique(entity, field, value)
	if errMongo != nil {
		// L'entité n'est pas mappée ou Mongo a crashé, on passe silencieusement au L3
	} else if existsMongo {
		return 0
	}

	// ÉTAPE 5 : POSTGRESQL (Cold Storage L3 - Source de Vérité)
	existsPg, errPg := postgres.FuncCheckUnique(ctx, entity, field, value)
	if errPg != nil {
		// Le mapper a bloqué la requête (entité inconnue) ou erreur SQL
		logger.Log.Error().Err(errPg).Str("entity", string(entity)).Str("field", field).Msg("Échec de la validation d'unicité")
		return 0 // SÉCURITÉ : Dans le doute, on refuse l'unicité pour éviter les doublons fatals.
	} else if existsPg {
		return 0
	}

	return 1
}
