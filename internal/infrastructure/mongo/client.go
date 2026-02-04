package mongo

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var MongoClient *mongo.Client

func InitMongo() {
	// 1. On récupère l'URI depuis le .env
	user := os.Getenv("MONGO_ROOT_USER")
	password := os.Getenv("MONGO_ROOT_PASSWORD")
	uri := "mongodb://" + user + ":" + password + "@mongo:27017"

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

func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	users := db.Collection("auth.users")
	user_settings := db.Collection("auth.user_settings")
	sessions := db.Collection("auth.sessions")
	relations := db.Collection("auth.relations")
	posts := db.Collection("content.posts")
	comments := db.Collection("content.comments")
	likes := db.Collection("content.likes")
	media := db.Collection("content.media")
	conversations := db.Collection("messaging.conversations")
	members := db.Collection("messaging.conversation_members")
	messages := db.Collection("messaging.messages")

	_, err1 := users.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "username", Value: 1}, {Key: "email", Value: 1}, {Key: "phone", Value: 1}}}})
	_, err2 := user_settings.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "user_id", Value: 1}}}})
	_, err3 := sessions.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "device_token", Value: 1}}}})
	_, err4 := relations.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "primary_id", Value: 1}, {Key: "secondary_id", Value: 1}}}})
	_, err5 := posts.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "hashtags", Value: 1}, {Key: "identifiers", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}}})
	_, err6 := comments.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "post_id", Value: 1}, {Key: "created_at", Value: -1}}}})
	_, err7 := likes.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "target_type", Value: 1}, {Key: "target_id", Value: 1}, {Key: "user_id", Value: 1}}}})
	_, err8 := media.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "owner_id", Value: 1}, {Key: "created_at", Value: -1}}}})
	_, err9 := conversations.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "last_message_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "state", Value: 1}}}})
	_, err10 := members.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "user_id", Value: 1}}}})
	_, err11 := messages.Indexes().CreateMany(ctx, []mongo.IndexModel{{Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}}}})
	return errors.Join(err1, err2, err3, err4, err5, err6, err7, err8, err9, err10, err11)
}
