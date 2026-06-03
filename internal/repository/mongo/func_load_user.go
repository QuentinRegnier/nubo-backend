package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

func MongoLoadUser(ID int64, Username string, Email string, Phone string) (models.UserRequest, error) {
	fmt.Println("MongoLoadUser called with:", ID, Username, Email, Phone)
	var u models.UserRequest

	// Construction du filtre de recherche
	filter := make(map[string]interface{})
	if ID != -1 && ID != 0 {
		filter["id"] = ID // ID Snowflake
	} else if Email != "" {
		filter["email"] = Email
	} else if Username != "" {
		filter["username"] = Username
	} else if Phone != "" {
		filter["phone"] = Phone
	} else {
		return models.UserRequest{}, fmt.Errorf("aucun critère de recherche mongo")
	}

	fmt.Println("MongoLoadUser filter:", filter)

	if len(filter) == 0 {
		return u, fmt.Errorf("MongoLoadUser: no research criteria (id, username, email, phone) provided")
	}

	// Appel à ta fonction utilitaire existante
	docs, err := Users.Get(filter, nil)
	if err != nil {
		return u, err
	}

	if len(docs) == 0 {
		return u, nil // Retourne une structure vide, pas d'erreur.
	}

	// Conversion Map -> Struct
	if err := pkg.ToStruct(docs[0], &u); err != nil {
		return u, err
	}

	return u, nil
}
