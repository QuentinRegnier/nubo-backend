package service

import (
	"context"
	"strings"

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

	// ÉTAPE 3 : SPEED CACHE (Vérification L1)
	// Pour les pseudos, on interroge l'index Lexicographique RAM pour un blocage absolu sans BDD.
	if entity == redis.EntityUser && field == "username" {
		lexValue := strings.ToLower(value)
		// ZRangeByLex cherche la valeur exacte en O(log N)
		res, err := redis.UsersLex.ZRangeByLex(ctx, "lex", lexValue, 1)
		if err == nil && len(res) > 0 {
			// Le format stocké est "username:id"
			if strings.Split(res[0], ":")[0] == lexValue {
				return 0 // Existe déjà dans le L1
			}
		}
	}

	// ÉTAPE 4 : MONGODB (Warm Storage L2)
	existsMongo, errMongo := mongo.MongoCheckUnique(entity, field, value)
	if errMongo == nil && existsMongo {
		// ⬆️ AUTO-GUÉRISON L1 : Le Cuckoo Filter l'avait oublié, on le répare
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, field, value)
		return 0
	}

	// ÉTAPE 5 : POSTGRESQL (Cold Storage L3 - Source de Vérité)
	existsPg, errPg := postgres.FuncCheckUnique(ctx, entity, field, value)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Str("entity", string(entity)).Str("field", field).Msg("Échec de la validation d'unicité")
		return 0 // SÉCURITÉ : Dans le doute, on refuse l'unicité
	} else if existsPg {
		// ⬆️ AUTO-GUÉRISON L1 : On répare le Cuckoo Filter
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, field, value)
		return 0
	}

	return 1
}

// ─────────────────────────────────────────────────────────────────────────────
// INITIALISATION & WARM-UP DU CUCKOO FILTER
// ─────────────────────────────────────────────────────────────────────────────

// WarmUpCuckooFilter lance le filtre en mémoire et le charge avec les données de Postgres.
// Cette fonction orchestre proprement le Repository et l'Infrastructure sans créer de cycle.
func WarmUpCuckooFilter(ctx context.Context) {
	// 1. Allouer la mémoire et démarrer la synchronisation
	cuckoo.InitCuckooFilter()

	logger.Log.Info().Msg("Chargement des données Postgres dans le Cuckoo Filter...")

	// 2. Récupérer la vérité absolue depuis la source (Cold Storage)
	identifiers, err := postgres.FuncLoadAllUserIdentifiers(ctx)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Erreur critique init Cuckoo (SQL via Repository)")
	}

	// 3. Peupler le filtre d'infrastructure
	count := 0
	for _, idents := range identifiers {
		if idents.Username != nil && *idents.Username != "" {
			cuckoo.GlobalCuckoo.Insert([]byte("username:" + *idents.Username))
		}
		if idents.Email != nil && *idents.Email != "" {
			cuckoo.GlobalCuckoo.Insert([]byte("email:" + *idents.Email))
		}
		if idents.Phone != nil && *idents.Phone != "" {
			cuckoo.GlobalCuckoo.Insert([]byte("phone:" + *idents.Phone))
		}
		count++
	}
	logger.Log.Info().Int("count", count).Msg("Cuckoo Filter chargé avec des utilisateurs (x3 clés).")
}
