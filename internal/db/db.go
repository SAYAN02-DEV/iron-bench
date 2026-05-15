package db

import (
	"log"

	"github.com/SAYAN02-DEV/iron-bench/internal/config"
	"github.com/SAYAN02-DEV/iron-bench/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(cfg *config.Config) {
	connStr := cfg.DatabaseURL
	if connStr == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	var err error
	DB, err = gorm.Open(postgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Fatal("failed to connect:", err)
	}

	DB.AutoMigrate(&models.User{})

	log.Println("Connected to database")
}

func Close() {
	if DB == nil {
		return
	}
	sqlDB, err := DB.DB()
	if err != nil {
		log.Println("failed to get sql DB:", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Println("failed to close DB:", err)
	}
}
