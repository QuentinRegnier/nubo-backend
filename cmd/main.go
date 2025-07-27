package main

import (
	"log"
	"os"

	"github.com/QuentinRegnier/nubo-backend/internal/api"
	"github.com/QuentinRegnier/nubo-backend/internal/cache"
	"github.com/QuentinRegnier/nubo-backend/internal/websocket"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialiser Redis
	cache.InitRedis()

	// Initialiser le Hub et lancer sa boucle
	websocket.InitHub()

	r := gin.Default()
	api.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("Server listening on %s", port)
	r.Run(":" + port)
}
