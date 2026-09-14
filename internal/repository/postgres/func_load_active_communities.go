package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncLoadActiveCommunities appelle la fonction SQL pour récupérer les communautés publiques
func FuncLoadActiveCommunities(ctx context.Context) ([]lite_models.CommunityLiteRequest, error) {
	query := `SELECT id, name, description, avatar_id, member_count FROM messaging.func_load_active_communities()`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var communities []lite_models.CommunityLiteRequest

	for rows.Next() {
		var c lite_models.CommunityLiteRequest
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.ProfilePictureID, &c.MemberCount); err == nil {
			communities = append(communities, c)
		}
	}

	return communities, nil
}
