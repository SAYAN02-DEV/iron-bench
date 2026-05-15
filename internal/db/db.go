package db

import (
	"context"
	"log"

	"github.com/SAYAN02-DEV/iron-bench/internal/config"
	"github.com/SAYAN02-DEV/iron-bench/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool
var Store *sqlc.Queries

func Connect(cfg *config.Config) {
	connStr := cfg.DatabaseURL
	if connStr == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("failed to connect:", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("failed to ping database:", err)
	}

	Pool = pool
	Store = sqlc.New(pool)

	log.Println("Connected to database")
}

func Close() {
	if Pool == nil {
		return
	}
	Pool.Close()
}
