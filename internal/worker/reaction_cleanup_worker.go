package worker

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"go.mongodb.org/mongo-driver/bson"
)

// StartReactionCleanupCron lance le Garbage Collector qui détruit les réactions orphelines.
func StartReactionCleanupCron(ctx context.Context) {
	logger.Log.Info().Msg("Démarrage du Garbage Collector de Réactions (6h)...")
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processReactionCleanup(ctx)
			}
		}
	}()
}

func processReactionCleanup(ctx context.Context) {
	// 1. Purge via PostgreSQL (L3)
	// Nécessitera : DELETE FROM messaging.message_reactions r WHERE NOT EXISTS (SELECT 1 FROM messaging.messages m WHERE m.id = r.message_id AND m.visibility = TRUE) RETURNING r.message_id;
	orphanMessageIDs, err := postgres.FuncDeleteOrphanReactions(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Garbage Collector Reactions : Erreur Postgres")
		return
	}

	if len(orphanMessageIDs) == 0 {
		return
	}

	logger.Log.Info().Int("count", len(orphanMessageIDs)).Msg("Garbage Collector : Réactions orphelines purgées de Postgres.")

	// 2. Purge du L2 (Mongo)
	if mongo.MessageReactions != nil {
		_, err := mongo.MessageReactions.DB.Collection(mongo.MessageReactions.Name).DeleteMany(ctx, bson.M{
			"message_id": bson.M{"$in": orphanMessageIDs},
		})
		if err != nil {
			logger.Log.Error().Err(err).Msg("Garbage Collector Reactions : Échec suppression Mongo")
		}
	}
}
