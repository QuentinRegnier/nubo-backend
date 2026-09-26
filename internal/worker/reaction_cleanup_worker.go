package worker

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"go.mongodb.org/mongo-driver/bson"
)

// ############################################################################
// # WORKER : REACTION CLEANUP (GARBAGE COLLECTOR DES RÉACTIONS)
// ############################################################################

// StartReactionCleanupCron lance le Garbage Collector qui détruit les réactions orphelines.
// Une réaction devient orpheline lorsque le message parent a été supprimé ou est invisible.
func StartReactionCleanupCron(ctx context.Context) {
	logger.Log.Info().Msg("Démarrage du Garbage Collector de Réactions (Cron 6h)...")

	go func() {
		ticker := time.NewTicker(variables.ReactionCleanupCronInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return // Arrêt gracieux
			case <-ticker.C:
				processReactionCleanup(ctx)
			}
		}
	}()
}

// processReactionCleanup orchestre la purge des réactions en base de données.
func processReactionCleanup(ctx context.Context) {
	// ── ÉTAPE 1 : IDENTIFICATION ET PURGE L3 (POSTGRESQL - SOURCE DE VÉRITÉ) ──
	// La fonction SQL vérifie l'existence et la visibilité du message parent.
	// Elle supprime les réactions orphelines et nous retourne la liste des messages impactés.
	orphanMessageIDs, errPg := postgres.FuncDeleteOrphanReactions(ctx)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Msg("Garbage Collector Réactions : Échec critique de la purge PostgreSQL (L3)")
		return
	}

	if len(orphanMessageIDs) == 0 {
		return // Rien à nettoyer ce cycle
	}

	logger.Log.Info().Int("count", len(orphanMessageIDs)).Msg("Garbage Collector Réactions : Purge L3 terminée. Répercussion sur le L2 en cours...")

	// ── ÉTAPE 2 : PURGE DU WARM STORAGE L2 (MONGODB) ──────────────────────────
	// On répercute la suppression sur Mongo pour éviter des réactions fantômes
	// en cas de Fallback L3 -> L2. (Le L1 est généralement géré dynamiquement par l'application).
	if mongo.MessageReactions != nil {
		_, errMongo := mongo.MessageReactions.DB.Collection(mongo.MessageReactions.Name).DeleteMany(ctx, bson.M{
			"message_id": bson.M{"$in": orphanMessageIDs},
		})

		if errMongo != nil {
			logger.Log.Error().Err(errMongo).Msg("Garbage Collector Réactions : Échec de la suppression sur MongoDB (L2)")
		}
	}
}
