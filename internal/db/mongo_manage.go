package db

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

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

	schemaUsers := UsersSchema
	schemaUserSettings := UserSettingsSchema
	schemaSessions := SessionsSchema
	schemaRelations := RelationsSchema
	schemaPosts := PostsSchema
	schemaComments := CommentsSchema
	schemaLikes := LikesSchema
	schemaMedia := MediaSchema
	schemaConversations := ConversationsSchema
	schemaMembers := MembersSchema
	schemaMessages := MessagesSchema

	// variables globales
	Users = NewMongoCollection("nubo_mongo", "auth.users", schemaUsers)
	UserSettings = NewMongoCollection("nubo_mongo", "auth.user_settings", schemaUserSettings)
	Sessions = NewMongoCollection("nubo_mongo", "auth.sessions", schemaSessions)
	Relations = NewMongoCollection("nubo_mongo", "auth.relations", schemaRelations)
	Posts = NewMongoCollection("nubo_mongo", "content.posts", schemaPosts)
	Comments = NewMongoCollection("nubo_mongo", "content.comments", schemaComments)
	Likes = NewMongoCollection("nubo_mongo", "content.likes", schemaLikes)
	Media = NewMongoCollection("nubo_mongo", "content.media", schemaMedia)
	ConversationsMeta = NewMongoCollection("nubo_mongo", "messaging.conversations", schemaConversations)
	ConversationMembers = NewMongoCollection("nubo_mongo", "messaging.conversation_members", schemaMembers)
	Messages = NewMongoCollection("nubo_mongo", "messaging.messages", schemaMessages)

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

// validate vérifie les types.
// partial = true : permet de ne vérifier QUE les champs présents (pour Update)
func (c *MongoCollection) validate(obj map[string]any, partial bool) error {
	// 1. Si validation complète exigée, on vérifie qu'il ne manque rien
	if !partial {
		for field := range c.Schema {
			if _, ok := obj[field]; !ok {
				return fmt.Errorf("champ manquant: %s", field)
			}
		}
	}

	// 2. Vérification des types pour les champs qui sont présents
	for field, val := range obj {
		expectedKind, known := c.Schema[field]
		if !known {
			continue // Champ hors schéma, on ignore
		}

		if reflect.TypeOf(val).Kind() != expectedKind {
			return fmt.Errorf("type invalide pour %s: attendu %s, reçu %s",
				field, expectedKind, reflect.TypeOf(val).Kind())
		}
	}
	return nil
}

// Set insère ou met à jour un objet dans la collection
func (c *MongoCollection) Set(obj map[string]any) error {
	// false = on veut valider que TOUS les champs sont là
	if err := c.validate(obj, false); err != nil {
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
	// true = on valide seulement les champs qu'on veut mettre à jour
	if err := c.validate(update, true); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := c.DB.Collection(c.Name)
	_, err := collection.UpdateMany(ctx, filter, bson.M{"$set": update})
	return err
}
