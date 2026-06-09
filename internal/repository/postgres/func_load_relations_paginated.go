package postgres

import (
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// RelationSeedPayload structure temporaire pour l'amorçage
type RelationSeedPayload struct {
	CallerID int64
	TargetID int64
	State    int
}

// FuncLoadRelationsPaginated appelle la fonction SQL auth.func_load_relations_paginated
func FuncLoadRelationsPaginated(limit, offset int) ([]RelationSeedPayload, error) {
	query := `SELECT * FROM auth.func_load_relations_paginated($1, $2)`
	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur fermeture rows dans FuncLoadRelationsPaginated:", err)
		}
	}(rows)

	var relations []RelationSeedPayload
	for rows.Next() {
		var rel RelationSeedPayload
		// primary_id = Caller, secondary_id = Target
		if err := rows.Scan(&rel.CallerID, &rel.TargetID, &rel.State); err == nil {
			relations = append(relations, rel)
		}
	}
	return relations, nil
}
