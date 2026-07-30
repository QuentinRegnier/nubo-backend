package auth_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
)

// GetUserSessions récupère les sessions actives et les expurge de toute donnée critique.
func GetUserSessions(ctx context.Context, userID int64) ([]auth_models.SessionView, error) {
	// 1. Lecture depuis la Source of Truth (PostgreSQL)
	sessions, err := postgres.FuncLoadUserSessionsView(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. Conversion et nettoyage sécurisé
	var views []auth_models.SessionView
	for _, s := range sessions {
		views = append(views, auth_models.SessionView{
			ID:         s.ID,
			DeviceInfo: s.DeviceInfo,
			IPHistory:  s.IPHistory,
			CreatedAt:  s.CreatedAt,
			ExpiresAt:  s.ExpiresAt,
		})
	}

	// Prévention du retour 'null' en JSON si aucune session
	if views == nil {
		views = make([]auth_models.SessionView, 0)
	}

	return views, nil
}
