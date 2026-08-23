package worker

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// StartMediaCleanupCron lance le Garbage Collector qui détruit les médias orphelins.
func StartMediaCleanupCron(ctx context.Context) {
	log.Println("🧹 Démarrage du Garbage Collector de Médias (1h)...")

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processMediaCleanup(ctx)
			}
		}
	}()
}

func processMediaCleanup(ctx context.Context) {
	// 1. Purge et récupération via PostgreSQL (L3)
	orphans, err := postgres.FuncDeleteOrphanMedia(ctx)
	if err != nil {
		log.Printf("  [Garbage Collector] Erreur Postgres : %v", err)
		return
	}

	if len(orphans) == 0 {
		return // Rien à nettoyer
	}

	log.Printf("  [Garbage Collector] %d médias orphelins purgés de Postgres.", len(orphans))

	bucketName := os.Getenv("MINIO_BUCKET_NAME")
	if bucketName == "" {
		bucketName = "nubo-bucket"
	}

	var idsToDelete []int64

	// 2. Traitement des effets de bord (S3 et RAM L1)
	for _, orphan := range orphans {
		idsToDelete = append(idsToDelete, orphan.ID)

		// A. Destruction physique du fichier sur MinIO via le domaine dédié (DDD)
		if orphan.StoragePath != "" {
			errS3 := media_service.RemovePhysicalMedia(ctx, orphan.StoragePath)
			if errS3 != nil {
				log.Printf("  [Garbage Collector] Échec S3 pour %s: %v", orphan.StoragePath, errS3)
			}
		}

		// B. Destruction de l'empreinte en RAM (L1)
		_ = object_cache_service.DeleteMediaFromObjectCache(ctx, orphan.ID)
	}

	// 3. Purge du Cold Storage (MongoDB L2)
	if err := mongo.MongoDeleteMediaByIDs(idsToDelete); err != nil {
		log.Printf("  [Garbage Collector] Échec suppression Mongo : %v", err)
	}

	log.Printf("  [Garbage Collector] Nettoyage complet (L1, L2, L3, S3) terminé pour %d médias.", len(orphans))
}
