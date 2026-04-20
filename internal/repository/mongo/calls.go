package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

func MongoLoadUser(ID int64, Username string, Email string, Phone string) (domain.UserRequest, error) {
	fmt.Println("MongoLoadUser called with:", ID, Username, Email, Phone)
	var u domain.UserRequest

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
		return domain.UserRequest{}, fmt.Errorf("aucun critère de recherche mongo")
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
func MongoLoadSession(ID int64, DeviceToken string, MasterToken string, CurrentSecret string) (domain.SessionsRequest, error) {
	fmt.Println("MongoLoadSession called with:", ID, DeviceToken, MasterToken, CurrentSecret)
	var s domain.SessionsRequest

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

// MongoLoadPosts récupère une liste de posts en fonction de leurs IDs (Niveau 2 Fallback)
func MongoLoadPosts(ids []int64) ([]domain.PostRequest, error) {
	if len(ids) == 0 {
		return []domain.PostRequest{}, nil
	}

	filter := map[string]any{
		"id": map[string]any{"$in": ids},
	}

	docs, err := Posts.Get(filter, nil)
	if err != nil {
		return nil, err
	}

	var posts []domain.PostRequest
	for _, doc := range docs {
		var p domain.PostRequest
		if err := pkg.ToStruct(doc, &p); err == nil {
			posts = append(posts, p)
		}
	}

	return posts, nil
}

// MongoLoadPostsPaginated récupère des posts avec filtres, tri et pagination (Cold Storage)
func MongoLoadPostsPaginated(filter map[string]any, sort map[string]any, skip int64, limit int64) ([]domain.PostRequest, error) {
	docs, err := Posts.GetPaginated(filter, sort, skip, limit)
	if err != nil {
		return nil, err
	}

	var posts []domain.PostRequest
	for _, doc := range docs {
		var p domain.PostRequest
		if err := pkg.ToStruct(doc, &p); err == nil {
			posts = append(posts, p)
		}
	}

	return posts, nil
}
