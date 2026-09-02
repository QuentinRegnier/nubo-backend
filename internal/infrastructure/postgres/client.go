package postgres

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	_ "github.com/lib/pq"
)

var PostgresDB *sql.DB

func InitPostgres() {
	host := os.Getenv("POSTGRES_HOST")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	port := os.Getenv("POSTGRES_PORT")

	if host == "" {
		host = "postgres"
	}
	if port == "" {
		port = "5432"
	}

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname,
	)

	var err error
	PostgresDB, err = sql.Open("postgres", connStr)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Erreur connexion PostgreSQL")
	}

	if err := PostgresDB.Ping(); err != nil {
		logger.Log.Fatal().Err(err).Msg("Ping PostgreSQL échoué")
	}
}
