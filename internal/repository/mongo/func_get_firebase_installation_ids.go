package mongo

import "go.mongodb.org/mongo-driver/bson"

// MongoGetfirebaseInstallationIDs récupère tous les tokens de notification d'un utilisateur dans le stockage à froid
func MongoGetFirebaseInstallationIDs(userID int64) ([]string, error) {
	if Sessions == nil {
		return nil, nil
	}

	filter := bson.M{"user_id": userID}
	docs, err := Sessions.Get(filter, nil)
	if err != nil {
		return nil, err
	}

	var tokens []string
	for _, doc := range docs {
		if token, ok := doc["firebase_installation_id"].(string); ok && token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}
