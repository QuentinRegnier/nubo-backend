package mongo

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoUpsertPost insère ou met à jour un payload complet de publication dans le Cold Storage L2.
// Cette fonction est indispensable lors de la remontée en cascade (L3 PostgreSQL -> L2 MongoDB)
// pour garantir la cohérence des lectures asynchrones ultérieures.
func MongoUpsertPost(post post_models.PostPayload) error {
	// Sécurité si l'initialisation globale de la collection Mongo a échoué
	if Posts == nil {
		return nil
	}

	// Context court à 5 secondes dédié aux opérations d'écriture d'infrastructure
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filtrage sur la clé primaire (Snowflake ID)
	filter := bson.M{"id": post.ID}

	// Écrase l'intégralité du document avec les valeurs fraîches de PostgreSQL
	update := bson.M{"$set": post}

	// Configuration atomique : Upsert = True
	opts := options.Update().SetUpsert(true)

	_, err := Posts.DB.Collection(Posts.Name).UpdateOne(ctx, filter, update, opts)
	return err
}
