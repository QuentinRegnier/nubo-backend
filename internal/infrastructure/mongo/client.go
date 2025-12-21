package mongo

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var MongoClient *mongo.Client

func InitMongo() {
	// 1. On récupère l'URI depuis le .env
	uri := os.Getenv("MONGO_URI")

	// SÉCURITÉ : Si vide, on met une valeur par défaut, MAIS on prévient
	if uri == "" {
		log.Println("⚠️ ATTENTION : MONGO_URI vide, fallback sur localhost (ça plantera dans Docker !)")
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. On se connecte
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("❌ Impossible de créer le client Mongo: %v", err)
	}

	// 3. Ping pour vérifier que ça marche VRAIMENT
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("❌ Impossible de ping Mongo (%s): %v", uri, err)
	}

	MongoClient = client
	log.Println("✅ Connecté à MongoDB avec succès !")
}
