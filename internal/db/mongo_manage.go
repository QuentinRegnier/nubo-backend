package db

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/tools"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---------------- Initialisation ----------------
// declarations globales
var (
	Users               *MongoCollection
	UserSettings        *MongoCollection
	Sessions            *MongoCollection
	Relations           *MongoCollection
	Posts               *MongoCollection
	Comments            *MongoCollection
	Likes               *MongoCollection
	Media               *MongoCollection
	ConversationsMeta   *MongoCollection
	ConversationMembers *MongoCollection
	Messages            *MongoCollection
)

// InitCacheDatabase initialise la structure logique de Redis pour les caches
func InitCacheDatabase() {
	// Initialiser les collections

	schemaUsers := tools.UsersSchema
	schemaUserSettings := tools.UserSettingsSchema
	schemaSessions := tools.SessionsSchema
	schemaRelations := tools.RelationsSchema
	schemaPosts := tools.PostsSchema
	schemaComments := tools.CommentsSchema
	schemaLikes := tools.LikesSchema
	schemaMedia := tools.MediaSchema
	schemaConversationsMeta := tools.ConversationsMetaSchema
	schemaConversationMembers := tools.ConversationMembersSchema
	schemaMessages := tools.MessagesSchema

	// variables globales
	Users = NewMongoCollection("nubo_db", "users", schemaUsers)
	UserSettings = NewMongoCollection("nubo_db", "user_settings", schemaUserSettings)
	Sessions = NewMongoCollection("nubo_db", "sessions", schemaSessions)
	Relations = NewMongoCollection("nubo_db", "relations", schemaRelations)
	Posts = NewMongoCollection("nubo_db", "posts", schemaPosts)
	Comments = NewMongoCollection("nubo_db", "comments", schemaComments)
	Likes = NewMongoCollection("nubo_db", "likes", schemaLikes)
	Media = NewMongoCollection("nubo_db", "media", schemaMedia)
	ConversationsMeta = NewMongoCollection("nubo_db", "conversations_meta", schemaConversationsMeta)
	ConversationMembers = NewMongoCollection("nubo_db", "conversation_members", schemaConversationMembers)
	Messages = NewMongoCollection("nubo_db", "messages", schemaMessages)

	log.Println("Structure MongoDB initialisée")
}

// ---------------- Collection et schéma ----------------

type MongoCollection struct {
	Name   string
	Schema map[string]reflect.Kind
	DB     *mongo.Database
}

// NewMongoCollection crée une collection Mongo avec un schéma
func NewMongoCollection(dbName, name string, schema map[string]reflect.Kind) *MongoCollection {
	return &MongoCollection{
		Name:   name,
		Schema: schema,
		DB:     MongoClient.Database(dbName),
	}
}

// validate vérifie qu'un objet correspond au schéma de la collection
func (c *MongoCollection) validate(obj map[string]any) error {
	for field, kind := range c.Schema {
		val, ok := obj[field]
		if !ok {
			return fmt.Errorf("champ manquant: %s", field)
		}
		if reflect.TypeOf(val).Kind() != kind {
			return fmt.Errorf("type invalide pour %s: attendu %s, reçu %s",
				field, kind, reflect.TypeOf(val).Kind())
		}
	}
	return nil
}

// Set insère ou met à jour un objet dans la collection
func (c *MongoCollection) Set(obj map[string]any) error {
	if err := c.validate(obj); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := c.DB.Collection(c.Name)

	// upsert (si id existe déjà, on remplace)
	filter := bson.M{"id": obj["id"]}
	update := bson.M{"$set": obj}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// Get récupère les objets correspondant au filtre avec une projection optionnelle
func (c *MongoCollection) Get(filter map[string]any, projection map[string]any) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := c.DB.Collection(c.Name)

	opts := options.Find()
	if projection != nil {
		opts.SetProjection(projection)
	}

	cur, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results []map[string]any
	if err := cur.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// Delete supprime les objets correspondant au filtre
func (c *MongoCollection) Delete(filter map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := c.DB.Collection(c.Name)
	_, err := collection.DeleteMany(ctx, filter)
	return err
}

// Update met à jour les éléments correspondant au filtre avec les nouvelles valeurs fournies dans update
func (c *MongoCollection) Update(filter map[string]any, update map[string]any) error {
	if err := c.validate(update); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := c.DB.Collection(c.Name)
	_, err := collection.UpdateMany(ctx, filter, bson.M{"$set": update})
	return err
}
