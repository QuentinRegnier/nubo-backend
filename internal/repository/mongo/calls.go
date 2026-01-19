package mongo

import (
	"fmt"
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
func MongoLoadSession(ID int, DeviceToken string, MasterToken string, CurrentSecret string) (domain.SessionsRequest, error) {
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
func MongoUpdateSession(s domain.SessionsRequest) error {
	// 1. Conversion de la structure en Map pour l'update
	doc, err := pkg.ToMap(s)
	if err != nil {
		return err
	}

	// 2. On retire l'ID des champs à mettre à jour (clé primaire immuable)
	delete(doc, "id")

	// 3. Construction du filtre de recherche dynamique
	filter := make(map[string]any)

	// On ajoute les critères de recherche s'ils sont présents (non zéro/vide)
	if s.ID != 0 {
		filter["id"] = s.ID
	}
	if s.UserID != 0 {
		filter["user_id"] = s.UserID
	}
	if s.DeviceToken != "" {
		filter["device_token"] = s.DeviceToken
	}

	// Sécurité : on empêche une mise à jour globale si aucun filtre n'est défini
	if len(filter) == 0 {
		return fmt.Errorf("MongoLoadSession: no research criteria (id, device_token) provided")
	}

	// 4. Appel à ta fonction standardisée Update
	// Elle effectue un $set sur les champs restants dans 'doc' pour les documents correspondant au 'filter'
	return Sessions.Update(filter, doc)
}
