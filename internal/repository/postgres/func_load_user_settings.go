package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

// FuncLoadUserSettings charge les paramètres complets d'un utilisateur (incluant son ADN algorithmique)
func FuncLoadUserSettings(ctx context.Context, userID int64) (user_settings_models.UserSettingsPayload, error) {
	query := `SELECT id, user_id, privacy, notifications, display_and_content, telemetry_vector, telemetry_tags, telemetry_timestamp
	          FROM auth.func_load_user_settings(NULL, $1)`

	var s user_settings_models.UserSettingsPayload
	var privacyBytes, notifBytes, displayBytes []byte

	err := postgres.PostgresDB.QueryRowContext(ctx, query, userID).Scan(
		&s.ID,
		&s.UserID,
		&privacyBytes,
		&notifBytes,
		&displayBytes,
		pq.Array(&s.TelemetryVector),
		pq.Array(&s.TelemetryTags),
		&s.TelemetryTimestamp,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Ce n'est pas une erreur, l'utilisateur n'a simplement pas encore de settings
			return s, nil
		}
		return s, nubo_error.NewInternal(err)
	}

	// Conversion des JSONB en Map
	if len(privacyBytes) > 0 {
		_ = json.Unmarshal(privacyBytes, &s.Privacy)
	}
	if len(notifBytes) > 0 {
		_ = json.Unmarshal(notifBytes, &s.Notifications)
	}
	if len(displayBytes) > 0 {
		_ = json.Unmarshal(displayBytes, &s.DisplayAndContent)
	}

	return s, nil
}
