package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

func MongoLoadSession(ID int64, FirebaseInstallationID string, MasterToken string, CurrentSecret string) (auth_models.SessionsPayload, error) {
	var s auth_models.SessionsPayload

	// Construction du filtre de recherche (uniquement les valeurs valides)
	filter := make(map[string]any)

	if ID != -1 {
		filter["user_id"] = ID
	} else {
		filter["user_id"] = nil
	}

	if FirebaseInstallationID != "" {
		filter["firebase_installation_id"] = FirebaseInstallationID
	} else {
		filter["firebase_installation_id"] = nil
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
		return s, nubo_error.NewInternal(fmt.Errorf("MongoLoadSession: aucun critère de recherche fourni"))
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
