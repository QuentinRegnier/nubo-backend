package service

import (
	"fmt"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
)

// IsUnique vérifie l'unicité d'une valeur (0 = existe déjà, 1 = unique)
func IsUnique(collection *mongo.MongoCollection, field string, value any) int {

	valStr := fmt.Sprintf("%v", value)
	key := field + ":" + valStr

	// ---------------------------------------------------------
	// 0. CUCKOO FILTER (RAM Layer - O(1))
	// ---------------------------------------------------------
	// Premier check ultra-rapide.
	// Si le Cuckoo ne le trouve pas, c'est CERTAIN qu'il n'existe pas.
	// On évite Redis, Mongo et Postgres.
	if cuckoo.GlobalCuckoo != nil {
		if !cuckoo.GlobalCuckoo.Lookup([]byte(key)) {
			return 1 // Unique (certitude 100%)
		}
	}
	// Si trouvé ici -> C'est PEUT-ÊTRE un doublon (ou faux positif).
	// On continue les vérifications pour confirmer.

	// ---------------------------------------------------------
	// 1. REDIS (Cache Layer)
	// ---------------------------------------------------------
	// TODO: Faire un GET sur la clé "table:field:value"
	// Si hit -> return 0

	// ---------------------------------------------------------
	// 2. MONGODB
	// ---------------------------------------------------------
	filter := map[string]any{
		field: value,
	}
	projection := map[string]any{
		"id": 1,
	}
	// Get est dans generic.go dans le package mongo
	results, err := collection.Get(filter, projection)
	if err != nil {
		log.Printf("Erreur IsUnique (Mongo Get) : %v", err)
		return 0 // Sécurité
	}

	if len(results) > 0 {
		return 0 // Existe déjà
	}

	// ---------------------------------------------------------
	// 3. POSTGRESQL
	// ---------------------------------------------------------
	query := fmt.Sprintf("SELECT count(1) FROM %s WHERE %s = $1", collection.Name, field)
	var countSQL int
	err = postgres.PostgresDB.QueryRow(query, value).Scan(&countSQL)
	if err != nil {
		log.Printf("Erreur IsUnique (Postgres) : %v", err)
	}

	if countSQL > 0 {
		return 0 // Existe déjà
	}

	// ---------------------------------------------------------
	// Résultat final : La valeur est unique partout
	// ---------------------------------------------------------
	return 1
}
