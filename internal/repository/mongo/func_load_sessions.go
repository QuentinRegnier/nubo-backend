package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

func MongoLoadSession(ID int64, DeviceToken string, MasterToken string, CurrentSecret string) (models.SessionsRequest, error) {
	fmt.Println("MongoLoadSession called with:", ID, DeviceToken, MasterToken, CurrentSecret)
	var s models.SessionsRequest

	// Construction du filtre de recherche (uniquement les valeurs valides)
	filter := make(map[string]any)

	if ID != -1 {
		filter["user_id"] = ID
	} else {
		filter["user_id"] = nil
	}

	if DeviceToken != "" {
		filter["device_token"] = DeviceToken
	} else {
		filter["device_token"] = nil
	}

	if MasterToken != "" {
		filter["master_token"] = MasterToken
	} else {
		filter["master_token"] = nil
	}

	if CurrentSecret != "" {
		filter["current_secret"] = CurrentSecret
	} else {
		filter["current_secret"] = nil
	}

	if len(filter) == 0 {
		return s, fmt.Errorf("MongoLoadSession: no research criteria (id, device_token, master_token) provided")
	}

	// Appel à la fonction utilitaire
	docs, err := Sessions.Get(filter, nil)
	if err != nil {
		return s, err
	}

	if len(docs) == 0 {
		return s, nil // aucune session trouvée, pas une erreur
	}

	// Conversion Map -> Struct
	if err := pkg.ToStruct(docs[0], &s); err != nil {
		return s, err
	}

	return s, nil
}
