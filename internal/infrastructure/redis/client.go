package redis

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

var Rdb *redis.Client

// InitRedis initialise la connexion Redis avec un timeout de sécurité
func InitRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "", // pas de mot de passe par défaut
		DB:       0,
	})

	// Vérifier la connexion avec un timeout de 5 secondes
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := Rdb.Ping(ctx).Err()
	if err != nil {
		log.Fatalf("Impossible de se connecter à Redis: %v", err)
	}
	log.Println("Connexion à Redis réussie ✅")
}

// GetWithTimeout récupère une valeur avec un timeout custom
func GetWithTimeout(key string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Rdb.Get(ctx, key).Result()
}

// SetWithTimeout ajoute une valeur avec un timeout custom
func SetWithTimeout(key string, value interface{}, expiration time.Duration, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Rdb.Set(ctx, key, value, expiration).Err()
}
