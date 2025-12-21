package mongo

import (
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

func MongoCreateUser(u domain.UserRequest) error {
	// Gestion des dates par défaut
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	u.UpdatedAt = time.Now()

	// Conversion Struct -> Map pour utiliser ta méthode générique .Set()
	doc, err := pkg.ToMap(u)
	if err != nil {
		return err
	}

	// Appel à ta fonction utilitaire existante
	return Users.Set(doc)
}

func MongoCreateSession(s domain.SessionsRequest) error {
	// Gestion des dates par défaut
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}

	// Conversion Struct -> Map pour utiliser ta méthode générique .Set()
	doc, err := pkg.ToMap(s)
	if err != nil {
		return err
	}

	// Appel à ta fonction utilitaire existante
	return Sessions.Set(doc)
}

func MongoLoadUser(ID int, Username string, Email string, Phone string) (domain.UserRequest, error) {
	var u domain.UserRequest

	// Construction du filtre de recherche
	filter := make(map[string]interface{})
	if ID != -1 {
		filter["id"] = ID
	} else {
		filter["id"] = nil
	}
	if Username != "" {
		filter["username"] = Username
	} else {
		filter["username"] = nil
	}
	if Email != "" {
		filter["email"] = Email
	} else {
		filter["email"] = nil
	}
	if Phone != "" {
		filter["phone"] = Phone
	} else {
		filter["phone"] = nil
	}

	// Appel à ta fonction utilitaire existante
	docs, err := Users.Get(filter, nil)
	if err != nil {
		return u, err
	}

	// Conversion Map -> Struct
	if err := pkg.ToStruct(docs[0], &u); err != nil {
		return u, err
	}

	return u, nil
}
func MongoLoadSession(ID int, DeviceToken string) (domain.SessionsRequest, error) {
	var s domain.SessionsRequest

	// Construction du filtre de recherche
	filter := make(map[string]interface{})
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

	// Appel à ta fonction utilitaire existante
	docs, err := Sessions.Get(filter, nil)
	if err != nil {
		return s, err
	}

	// Conversion Map -> Struct
	if err := pkg.ToStruct(docs[0], &s); err != nil {
		return s, err
	}

	return s, nil
}
