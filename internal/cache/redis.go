package cache

import (
	"context"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
)

var Ctx = context.Background()
var Rdb *redis.Client

func InitRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "", // pas de mot de passe par défaut
		DB:       0,
	})

	err := Rdb.Ping(Ctx).Err()
	if err != nil {
		log.Fatalf("Impossible de se connecter à Redis: %v", err)
	}
	log.Println("Connexion à Redis réussie")
}
