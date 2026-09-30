package mongo

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"go.mongodb.org/mongo-driver/bson"
)

// MongoGetfirebaseInstallationIDs récupère tous les tokens de notification d'un utilisateur dans le stockage à froid
func MongoGetFirebaseInstallationIDs(ctx context.Context, userID int64) ([]string, error) {
	if Sessions == nil {
		return nil, nil
	}

	filter := bson.M{"user_id": userID}
	docs, err := Sessions.Get(filter, nil)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return nil, nubo_error.NewInternal()
	}

	var tokens []string
	for _, doc := range docs {
		if token, ok := doc["firebase_installation_id"].(string); ok && token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}
