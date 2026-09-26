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
// # WORKER : SAVED CLEANUP (GARBAGE COLLECTOR DES SAUVEGARDES ORPHELINES)
// ############################################################################

// StartSavedCleanupCron lance le Garbage Collector qui détruit les favoris orphelins.
// Un post sauvegardé devient orphelin s'il est soft-deleted ou hard-deleted par son auteur.
func StartSavedCleanupCron(ctx context.Context) {
	logger.Log.Info().Msg("Démarrage du Garbage Collector de Favoris (Cron 6h)...")

	go func() {
		ticker := time.NewTicker(variables.SavedCleanupCronInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return // Arrêt gracieux
			case <-ticker.C:
				processSavedCleanup(ctx)
			}
		}
	}()
}

// processSavedCleanup orchestre la purge des favoris obsolètes en base de données.
func processSavedCleanup(ctx context.Context) {
	// ── ÉTAPE 1 : IDENTIFICATION ET PURGE L3 (POSTGRESQL - SOURCE DE VÉRITÉ) ──
	// PostgreSQL exécute la jointure pour trouver les enregistrements de "save"
	// dont le post parent n'existe plus ou est masqué (visibility < 0 ou = 2).
	orphanPostIDs, errPg := postgres.FuncDeleteOrphanSaved(ctx)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Msg("Garbage Collector Favoris : Échec critique de la purge PostgreSQL (L3)")
		return
	}

	if len(orphanPostIDs) == 0 {
		return // Rien à nettoyer ce cycle
	}

	logger.Log.Info().Int("count", len(orphanPostIDs)).Msg("Garbage Collector Favoris : Purge L3 terminée. Répercussion sur le L2 en cours...")

	// ── ÉTAPE 2 : PURGE DU WARM STORAGE L2 (MONGODB) ──────────────────────────
	// Afin de maintenir la cohérence des données et éviter les "PostNotFound" côté client,
	// on supprime physiquement les références orphelines du cache secondaire.
	if mongo.Saved != nil {
		_, errMongo := mongo.Saved.DB.Collection(mongo.Saved.Name).DeleteMany(ctx, bson.M{
			"post_id": bson.M{"$in": orphanPostIDs},
		})

		if errMongo != nil {
			logger.Log.Error().Err(errMongo).Msg("Garbage Collector Favoris : Échec de la suppression sur MongoDB (L2)")
		}
	}
}
