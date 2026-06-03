package mongo

import "fmt"

// MongoGetRelationState vérifie l'état de la relation dans le stockage à froid Mongo.
func MongoGetRelationState(callerID int64, targetID int64) (int, error) {
	// Filtre strict sur l'appelant et la cible
	filter := map[string]any{
		"caller_id": callerID,
		"target_id": targetID,
	}

	docs, err := Relations.GetPaginated(filter, nil, 0, 1)
	if err != nil || len(docs) == 0 {
		return 0, fmt.Errorf("relation introuvable dans mongo") // L'erreur déclenchera le fallback L3
	}

	// Extraction robuste et défensive de l'entier "state" depuis le BSON générique
	if stateFloat, ok := docs[0]["state"].(float64); ok {
		return int(stateFloat), nil
	}
	if stateInt32, ok := docs[0]["state"].(int32); ok {
		return int(stateInt32), nil
	}
	if stateInt64, ok := docs[0]["state"].(int64); ok {
		return int(stateInt64), nil
	}
	if stateInt, ok := docs[0]["state"].(int); ok {
		return stateInt, nil
	}

	return 0, fmt.Errorf("format de state invalide dans la collection relations")
}
