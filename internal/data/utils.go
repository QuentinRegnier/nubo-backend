package data

import (
	"fmt"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/db"
)

// IsUnique vérifie l'unicité d'une valeur (0 = existe déjà, 1 = unique)
func IsUnique(collection *db.MongoCollection, field string, value any) int {

	// ---------------------------------------------------------
	// 1. REDIS (Cache Layer)
	// ---------------------------------------------------------
	// TODO: Faire un GET sur la clé "table:field:value"
	// Si hit -> return 0

	// ---------------------------------------------------------
	// 2. MONGODB (Via ta fonction Get)
	// ---------------------------------------------------------

	// Filtre : on cherche l'élément avec ce champ et cette valeur
	filter := map[string]any{
		field: value,
	}

	// Projection : On ne récupère que le champ "id" pour être léger
	// (pas besoin de charger tout le profil utilisateur)
	projection := map[string]any{
		"id": 1,
	}

	// Appel de ta fonction existante
	results, err := collection.Get(filter, projection)

	if err != nil {
		log.Printf("Erreur IsUnique (Mongo Get) : %v", err)
		// En cas d'erreur technique, on renvoie 0 par sécurité (bloquant)
		// ou 1 si tu préfères laisser passer. Ici 0 évite les doublons accidentels.
		return 0
	}

	// Si la liste contient au moins un résultat, c'est que la valeur existe déjà
	if len(results) > 0 {
		return 0 // Pas unique
	}

	// ---------------------------------------------------------
	// 3. POSTGRESQL
	// ---------------------------------------------------------
	// On garde la vérification SQL directe car tu n'as pas fourni de wrapper "Get" pour Postgres
	// et PostgresDB est ta connexion globale sql.DB

	query := fmt.Sprintf("SELECT count(1) FROM %s WHERE %s = $1", collection.Name, field)

	var countSQL int
	err = db.PostgresDB.QueryRow(query, value).Scan(&countSQL)
	if err != nil {
		log.Printf("Erreur IsUnique (Postgres) : %v", err)
	}

	if countSQL > 0 {
		return 0 // Pas unique
	}

	// ---------------------------------------------------------
	// Résultat final : La valeur est unique partout
	// ---------------------------------------------------------
	return 1
}
