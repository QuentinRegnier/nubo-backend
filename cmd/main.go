package main

import (
	"log"
	"os"
	"time"

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
// @host            localhost:8080
// @BasePath        /api/v1
func main() {
	// Initialiser PostgreSQL
	postgres.InitPostgres()

	// Initialiser MongoDB
	mongo.InitMongo()

	// Initialiser Redis
	redis.InitRedis()

	// Nettoyage au démarrage
	service.InitData()

	// ⚡ Initialiser la stratégie Redis
	redisgo.GlobalStrategy = redisgo.NewLRUCache(redis.Rdb)

	// ⚡ Démarrer le watcher mémoire
	// maxRAM = 0 => autodétection
	// marge = 200 Mo de marge de sécurité
	// interval = toutes les 2 secondes
	redisgo.GlobalStrategy.StartMemoryWatcher(0, 200*1024*1024, 2*time.Second)

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

	r.GET("/swagger.json", func(c *gin.Context) {
		c.File("./docs/swagger.json")
	})

	// Servir l'interface HTML (Scalar)
	r.GET("/docs", func(c *gin.Context) {
		c.File("./docs.html")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on %s", port)
	log.Printf("v8 API ready")
	r.Run(":" + port)
}
