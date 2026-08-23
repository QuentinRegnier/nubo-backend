package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncGetFirebaseInstallationIDs récupère tous les tokens de notification actifs d'un utilisateur (Pur DDD)
func FuncGetFirebaseInstallationIDs(ctx context.Context, userID int64) ([]string, error) {
	query := `SELECT firebase_installation_id FROM auth.func_get_firebase_installation_ids($1)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("Error closing rows:", err)
		}
	}(rows)

	var fids []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil && t != "" {
			fids = append(fids, t)
		}
	}
	return fids, nil
}
