package db

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

var Conn *pgx.Conn

func Connect() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatal("failed to connect:", err)
	}

	Conn = conn
}
