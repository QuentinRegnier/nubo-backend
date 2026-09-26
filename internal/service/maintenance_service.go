package service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"go.mongodb.org/mongo-driver/bson"
)

// ############################################################################
// # MAINTENANCE ET NETTOYAGE DES CACHES (GARBAGE COLLECTION)
// ############################################################################

// CleanMongo effectue une purge temporelle glissante (Sliding TTL) sur le Warm Storage.
// Tous les documents non accédés ("last_use") depuis 30 jours sont évincés.
func CleanMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	recentDatabase := mongo.MongoClient.Database("nubo_recent")

	// 1. Découverte dynamique de toutes les collections
	collectionNames, err := recentDatabase.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Échec de la découverte des collections Mongo pour la purge")
		return
	}

	// 2. Le seuil de péremption : 30 jours dans le passé
	expirationThreshold := time.Now().AddDate(0, 0, -30)
	deletionFilter := bson.M{
		"last_use": bson.M{
			"$lt": expirationThreshold,
		},
	}

	// 3. Purge itérative
	for _, collectionName := range collectionNames {
		targetCollection := recentDatabase.Collection(collectionName)

		deletionResult, errDelete := targetCollection.DeleteMany(ctx, deletionFilter)
		if errDelete != nil {
			logger.Log.Error().Err(errDelete).Str("collection", collectionName).Msg("Échec de la purge du cache glissant Mongo")
			continue
		}

		if deletionResult.DeletedCount > 0 {
			logger.Log.Info().
				Str("collection", collectionName).
				Int64("deleted_count", deletionResult.DeletedCount).
				Msg("Purge glissante L2 Mongo réussie")
		}
	}
}

// CleanRedis vide l'intégralité du cache L1. Utilisé uniquement en environnement de Dev ou lors d'un Hard Reset.
func CleanRedis() {
	// Sécurité anti-crash unifiée via la couche d'accès
	if !redis.IsReady() {
		logger.Log.Warn().Msg("Nettoyage ignoré : Connexion Redis non initialisée (Rdb est nil).")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := redis.FlushDB(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Échec critique du Flush Redis")
		return
	}

	logger.Log.Info().Msg("Cache volatil Redis (L1) vidé avec succès.")
}

// InitData orchestre le grand nettoyage au démarrage du serveur si le flag CLEAN_DB_ON_STARTUP est actif.
func InitData() {
	logger.Log.Info().Msg("Début de la séquence de Hard Reset : Nettoyage L1 (Redis) et L2 (Mongo)...")
	CleanMongo()
	CleanRedis()
	logger.Log.Info().Msg("Séquence de Hard Reset terminée avec succès.")
}
