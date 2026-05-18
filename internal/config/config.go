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
	LongPollingSeconds string
	SQSURL           string
	SQSMaxMessages   string
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
		LongPollingSeconds: getEnv("LONG_POLLING_SECONDS", "10"),
		SQSURL:           os.Getenv("SQS_URL"),
		SQSMaxMessages:   getEnv("SQS_MAX_MESSAGES", "10"),
	}
	return cfg, err
}
