package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/docs"
	"github.com/QuentinRegnier/nubo-backend/internal/api"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/minio"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	mongogo "github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/worker"
	"github.com/gin-gonic/gin"
)

// @title           Mon API Propre
// @version         1.0
// @description     Documentation de l'API.
// @description Pour mettre à jour :
//
//	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/main.go -d . --parseDependency --parseInternal
//
// @BasePath        /api/v12
func main() {
	// --- INITIALISATION SNOWFLAKE ---

	// 1. On récupère la variable définie dans le docker-compose
	nodeIDStr := os.Getenv("NODE_ID")
	if nodeIDStr == "" {
		// Par sécurité, si tu oublies de le mettre, on prévient ou on met 0 par défaut
		log.Println("⚠️ ATTENTION : NODE_ID non défini, utilisation de 0 par défaut")
		nodeIDStr = "0"
	}

	// 2. On convertit le string "1" en int64 1
	nodeID, err := strconv.ParseInt(nodeIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Erreur: NODE_ID doit être un nombre entier. Reçu: %s", nodeIDStr)
	}

	// 3. On lance le moteur Snowflake
	err = pkg.InitSnowflake(nodeID)
	if err != nil {
		log.Fatalf("Impossible d'initialiser Snowflake: %v", err)
	}

	log.Printf("✅ Snowflake initialisé avec le Node ID : %d", nodeID)

	// Initialiser PostgreSQL
	postgres.InitPostgres()

	// Initialiser MongoDB
	mongo.InitMongo()

	// Initialiser Redis
	redis.InitRedis()

	// NOUVEAU : Initialiser les collections du Repository Redis
	redisgo.InitCacheDatabase()

	// Nettoyage au démarrage
	service.InitData()

	// Initialiser le Hub et lancer sa boucle
	//websocket.InitHub()

	// Initiatiser MinIO
	minio.InitMinio()

	// Iniitaliser la structure MongoDB
	mongogo.InitCacheDatabase()

	// Initialiser le Cuckoo Filter
	cuckoo.InitCuckooFilter()

	// --- NOUVEAU : CHARGEMENT DU CERVEAU DES TAGS ---
	err = service.LoadTagsConfig("assets/tags_config.json")
	if err != nil {
		log.Fatalf("❌ Impossible de charger la configuration des tags : %v", err)
	}

	// Lance le moteur V12
	worker.StartBackgroundWorkers(context.Background())

	r := gin.Default()
	api.SetupRoutes(r)

	// Initialiser la documentation
	docs.InitDocsRoutes(r)

	// Initialiser les Index Mongo
	_ = mongo.EnsureIndexes(context.Background(), mongo.MongoClient.Database("nubo"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on %s", port)
	log.Printf("v12 API ready")

	// SÉCURITÉ : Configuration stricte des Timeouts pour contrer Slowloris
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,  // Temps max pour lire la requête (Headers + Body)
		WriteTimeout: 10 * time.Second, // Temps max pour envoyer la réponse
		IdleTimeout:  15 * time.Second, // Temps max de maintien d'une connexion keep-alive
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Erreur fatale du serveur HTTP: %v", err)
	}
}
