package worker

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"go.mongodb.org/mongo-driver/bson"
)

// StartSavedCleanupCron lance le Garbage Collector qui détruit les favoris orphelins.
func StartSavedCleanupCron(ctx context.Context) {
	logger.Log.Info().Msg("Démarrage du Garbage Collector de Favoris (6h)...")
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processSavedCleanup(ctx)
			}
		}
	}()
}

func processSavedCleanup(ctx context.Context) {
	// 1. Purge via PostgreSQL (L3)
	// Nécessitera une fonction SQL : DELETE FROM content.saved s WHERE NOT EXISTS (SELECT 1 FROM content.posts p WHERE p.id = s.post_id AND p.visibility >= 0 AND p.visibility != 2) RETURNING s.post_id;
	orphanPostIDs, err := postgres.FuncDeleteOrphanSaved(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Garbage Collector Saved : Erreur Postgres")
		return
	}

	if len(orphanPostIDs) == 0 {
		return
	}

	logger.Log.Info().Int("count", len(orphanPostIDs)).Msg("Garbage Collector : Favoris orphelins purgés de Postgres.")

	// 2. Purge du L2 (Mongo)
	if mongo.Saved != nil {
		_, err := mongo.Saved.DB.Collection(mongo.Saved.Name).DeleteMany(ctx, bson.M{
			"post_id": bson.M{"$in": orphanPostIDs},
		})
		if err != nil {
			logger.Log.Error().Err(err).Msg("Garbage Collector Saved : Échec suppression Mongo")
		}
	}
}
