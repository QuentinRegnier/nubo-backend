package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

// FuncLoadUserSessionsView récupère la vue expurgée des sessions (Pur DDD)
func FuncLoadUserSessionsView(ctx context.Context, userID int64) ([]auth_models.SessionView, error) {
	// Appel direct à la nouvelle fonction SQL dédiée
	query := `SELECT id, device_info, ip_history, created_at, expires_at 
	          FROM auth.func_get_active_sessions_view($1)`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(rows)

	var sessions []auth_models.SessionView
	for rows.Next() {
		var s auth_models.SessionView
		var deviceInfoBytes []byte

		if err := rows.Scan(&s.ID, &deviceInfoBytes, pq.Array(&s.IPHistory), &s.CreatedAt, &s.ExpiresAt); err == nil {
			_ = json.Unmarshal(deviceInfoBytes, &s.DeviceInfo)
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}
