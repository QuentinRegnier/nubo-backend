package db

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

var PostgresDB *sql.DB

func InitPostgres() {
	connStr := "postgres://nubo_user:nubo_password@localhost:5432/nubo_db?sslmode=disable"
	var err error
	PostgresDB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Erreur connexion PostgreSQL :", err)
	}

	if err := PostgresDB.Ping(); err != nil {
		log.Fatal("Ping PostgreSQL échoué :", err)
	}
}
