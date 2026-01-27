package old

import (
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
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
	return mongo.Users.Set(doc)
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
	return mongo.Sessions.Set(doc)
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
	return mongo.Sessions.Update(filter, doc)
}
