package mongo

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoUpsertUser insère ou met à jour le profil complet d'un utilisateur dans le Cold Storage L2.
// Indispensable pour la remontée d'informations en cascade (L3 PostgreSQL -> L2 MongoDB)
// afin de synchroniser l'empreinte utilisateur lors d'un cache miss général.
func MongoUpsertUser(user auth_models.UserPayload) error {
	// Sécurité si l'initialisation de la collection globale a échoué au démarrage
	if Users == nil {
		return nil
	}

	// Isolation du contexte d'infrastructure pour l'écriture (5 secondes max)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filtrage sur la clé primaire de l'utilisateur
	filter := bson.M{"id": user.ID}

	// Écrase ou met à jour l'intégralité du document avec les valeurs fraîches de PostgreSQL
	update := bson.M{"$set": user}

	// Déclaration atomique du Upsert
	opts := options.Update().SetUpsert(true)

	_, err := Users.DB.Collection(Users.Name).UpdateOne(ctx, filter, update, opts)
	return err
}
