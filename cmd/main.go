package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/docs"
	"github.com/QuentinRegnier/nubo-backend/internal/api"
	"github.com/QuentinRegnier/nubo-backend/internal/api/websocket"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/minio"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	mongogo "github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// @title           Mon API Propre
// @version         1.0
// @description     Documentation de l'API.
// @description Pour mettre à jour :
//
//	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/main.go -d . --parseDependency --parseInternal
//
// @BasePath        /api/v11
func main() {
	// Initialiser PostgreSQL
	postgres.InitPostgres()

	// Initialiser MongoDB
	mongo.InitMongo()

	// Initialiser Redis
	redis.InitRedis()

	// Nettoyage au démarrage
	service.InitData()

	// ⚡ Démarrer le Sentinel Distribué (Mode Dynamique)
	// Argument 1 : Context
	// Argument 2 : Client Redis
	// Argument 3 : MARGE DE SÉCURITÉ (Ce qu'on doit laisser libre).
	//              Ex: 256MB. Redis prendra tout le reste disponible.
	// Argument 4 : Intervalle de vérification
	redisgo.StartMemorySentinel(context.Background(), redis.Rdb, 256*1024*1024, 2*time.Second)

	// Initialiser le Hub et lancer sa boucle
	websocket.InitHub()

	// Initiatiser MinIO
	minio.InitMinio()

	// Iniitaliser la structure MongoDB
	mongogo.InitCacheDatabase()

	// Initialiser la structure Redis (caches)
	redisgo.InitCacheDatabase()

	// Initialiser le Cuckoo Filter
	cuckoo.InitCuckooFilter()

	r := gin.Default()
	api.SetupRoutes(r)

	// Initialiser la documentation
	docs.InitDocsRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on %s", port)
	log.Printf("v11 API ready")
	r.Run(":" + port)
}
