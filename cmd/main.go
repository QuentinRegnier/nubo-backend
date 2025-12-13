package main

import (
	"log"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/api"
	"github.com/QuentinRegnier/nubo-backend/internal/cache"
	"github.com/QuentinRegnier/nubo-backend/internal/data"
	"github.com/QuentinRegnier/nubo-backend/internal/db"
	"github.com/QuentinRegnier/nubo-backend/internal/media"
	"github.com/QuentinRegnier/nubo-backend/internal/websocket"
	"github.com/gin-gonic/gin"
)

// @title           Mon API Propre
// @version         1.0
// @description     Documentation de l'API.
// @host            localhost:8080
// @BasePath        /api/v1
func main() {
	// Initialiser PostgreSQL
	db.InitPostgres()

	// Initialiser MongoDB
	db.InitMongo()

	// Initialiser Redis
	cache.InitRedis()

	// Nettoyage au démarrage
	data.InitData()

	// ⚡ Initialiser la stratégie Redis
	cache.GlobalStrategy = cache.NewLRUCache(cache.Rdb)

	// ⚡ Démarrer le watcher mémoire
	// maxRAM = 0 => autodétection
	// marge = 200 Mo de marge de sécurité
	// interval = toutes les 2 secondes
	cache.GlobalStrategy.StartMemoryWatcher(0, 200*1024*1024, 2*time.Second)

	// Initialiser le Hub et lancer sa boucle
	websocket.InitHub()

	// Initiatiser MinIO
	media.InitMinio()

	// Iniitaliser la structure MongoDB
	db.InitCacheDatabase()

	// Initialiser la structure Redis (caches)
	cache.InitCacheDatabase()

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
	log.Printf("v7 API ready")
	r.Run(":" + port)
}
