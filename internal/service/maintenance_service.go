package service

import (
	"context"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/mongo"
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
		log.Printf("❌ Erreur récupération collections Mongo: %v", err)
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
			log.Printf("❌ Erreur suppression dans %s: %v", collName, err)
			continue
		}

		log.Printf("🧹 Nettoyage Mongo [%s] → %d documents supprimés", collName, res.DeletedCount)
	}
}

func CleanRedis() {
	// Sécurité anti-crash unifiée via la couche d'accès
	if !redis.IsReady() {
		log.Println("⚠️ Redis n'est pas initialisé (Rdb est nil), nettoyage ignoré.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := redis.FlushDB(ctx)
	if err != nil {
		log.Printf("❌ Erreur flush Redis: %v", err)
		return
	}
	log.Println("🧹 Redis vidé avec succès ✅")
}

func InitData() {
	log.Println("=== Initialisation: Nettoyage Mongo + Redis ===")
	CleanMongo()
	CleanRedis()
	log.Println("=== Initialisation terminée ✅ ===")
}
