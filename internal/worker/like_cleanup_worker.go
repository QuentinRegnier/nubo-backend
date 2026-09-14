package worker

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"go.mongodb.org/mongo-driver/bson"
)

// StartLikeCleanupCron lance le Garbage Collector qui détruit les likes orphelins.
func StartLikeCleanupCron(ctx context.Context) {
	logger.Log.Info().Msg("Démarrage du Garbage Collector de Likes (6h)...")

	go func() {
		ticker := time.NewTicker(6 * time.Hour) // Tourne toutes les 6 heures
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processLikeCleanup(ctx)
			}
		}
	}()
}

func processLikeCleanup(ctx context.Context) {
	// 1. Purge et récupération via PostgreSQL (Source de vérité absolue L3)
	orphanTargets, err := postgres.FuncDeleteOrphanLikes(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Garbage Collector Likes : Erreur Postgres")
		return
	}

	if len(orphanTargets) == 0 {
		return // Rien à nettoyer
	}

	logger.Log.Info().Int("count", len(orphanTargets)).Msg("Garbage Collector : Likes orphelins purgés de Postgres.")

	// 2. Tri des cibles pour répercuter la suppression sur Mongo (L2)
	var postIDs []int64
	var commentIDs []int64

	for _, target := range orphanTargets {
		if target.TargetType == 0 {
			postIDs = append(postIDs, target.TargetID)
		} else if target.TargetType == 1 {
			commentIDs = append(commentIDs, target.TargetID)
		}
	}

	// 3. Purge du Warm Storage (MongoDB L2)
	if mongo.Likes != nil {
		if len(postIDs) > 0 {
			_, err := mongo.Likes.DB.Collection(mongo.Likes.Name).DeleteMany(ctx, bson.M{
				"target_type": 0,
				"target_id":   bson.M{"$in": postIDs},
			})
			if err != nil {
				logger.Log.Error().Err(err).Msg("Garbage Collector Likes : Échec suppression Mongo (Posts)")
			}
		}

		if len(commentIDs) > 0 {
			_, err := mongo.Likes.DB.Collection(mongo.Likes.Name).DeleteMany(ctx, bson.M{
				"target_type": 1,
				"target_id":   bson.M{"$in": commentIDs},
			})
			if err != nil {
				logger.Log.Error().Err(err).Msg("Garbage Collector Likes : Échec suppression Mongo (Commentaires)")
			}
		}
	}
}
