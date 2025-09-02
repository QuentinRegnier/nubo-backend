package main

import (
	"log"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/api"
	"github.com/QuentinRegnier/nubo-backend/internal/cache"
	"github.com/QuentinRegnier/nubo-backend/internal/db"
	"github.com/QuentinRegnier/nubo-backend/internal/initdata"
	"github.com/QuentinRegnier/nubo-backend/internal/websocket"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialiser PostgreSQL
	db.InitPostgres()

	// Initialiser MongoDB
	db.InitMongo()

	// Nettoyage au démarrage
	initdata.InitData()

	// Initialiser Redis
	cache.InitRedis()

	// ⚡ Initialiser la stratégie Redis
	cache.GlobalStrategy = cache.NewLRUCache(cache.Rdb)

	// ⚡ Démarrer le watcher mémoire
	// maxRAM = 0 => autodétection
	// marge = 200 Mo de marge de sécurité
	// interval = toutes les 2 secondes
	cache.GlobalStrategy.StartMemoryWatcher(0, 200*1024*1024, 2*time.Second)

	// Initialiser le Hub et lancer sa boucle
	websocket.InitHub()

	r := gin.Default()
	api.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on %s", port)
	log.Printf("v7 API ready")
	r.Run(":" + port)
}
