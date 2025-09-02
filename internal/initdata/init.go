package initdata

import (
	"context"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/cache"
	"github.com/QuentinRegnier/nubo-backend/internal/db"
	"go.mongodb.org/mongo-driver/bson"
)

func CleanMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dbRecent := db.MongoClient.Database("nubo_recent")

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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cache.Rdb.FlushDB(ctx).Err()
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
