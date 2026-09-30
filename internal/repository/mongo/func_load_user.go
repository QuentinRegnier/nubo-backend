package mongo

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

func MongoLoadUser(ctx context.Context, ID int64, Username string, Email string, Phone string) (auth_models.UserPayload, error) {
	var u auth_models.UserPayload

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
		nubo_log.Error(ctx).Msg("Construction du filtre Mongo échouée : aucun critère de recherche valide fourni")
		return auth_models.UserPayload{}, nubo_error.NewInternal()
	}

	if len(filter) == 0 {
		nubo_log.Error(ctx).Msg("MongoLoadUser : Aucun critère de recherche fourni pour charger l'utilisateur")
		return u, nubo_error.NewInternal()
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
