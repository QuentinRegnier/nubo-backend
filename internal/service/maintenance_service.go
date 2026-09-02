package service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"go.mongodb.org/mongo-driver/bson"
)

func CleanMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dbRecent := mongo.MongoClient.Database("nubo_recent")

	// Récupère toutes les collections de la DB
	collections, err := dbRecent.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Erreur récupération collections Mongo")
		return
	}

	// Date limite : 30 jours
	threshold := time.Now().AddDate(0, 0, -30)

	for _, collName := range collections {
		coll := dbRecent.Collection(collName)

		// Supprime les documents dont last_use < threshold
		filter := bson.M{
			"last_use": bson.M{
				"$lt": threshold,
			},
		}

		res, err := coll.DeleteMany(ctx, filter)
		if err != nil {
			logger.Log.Error().Err(err).Str("collection", collName).Msg("Erreur de suppression du cache glissant Mongo")
			continue
		}

		logger.Log.Info().Str("collection", collName).Int64("deleted_count", res.DeletedCount).Msg("Nettoyage Mongo réussi")
	}
}

func CleanRedis() {
	// Sécurité anti-crash unifiée via la couche d'accès
	if !redis.IsReady() {
		logger.Log.Warn().Msg("Redis n'est pas initialisé (Rdb est nil), nettoyage ignoré.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := redis.FlushDB(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Erreur flush Redis")
		return
	}
	logger.Log.Info().Msg("Redis vidé avec succès")
}

func InitData() {
	logger.Log.Info().Msg("Début de l'initialisation : Nettoyage Mongo + Redis")
	CleanMongo()
	CleanRedis()
	logger.Log.Info().Msg("Initialisation terminée avec succès")
}
