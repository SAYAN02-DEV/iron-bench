package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL      string
	Port             string
	JWTSecret        string
	OrchestratorPort string
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func Load() (*Config, error) {
	err := godotenv.Load()
	cfg := &Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		Port:             getEnv("PORT", "8080"),
		OrchestratorPort: getEnv("ORCHESTRATOR_PORT", "8000"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
	}
	return cfg, err
}
