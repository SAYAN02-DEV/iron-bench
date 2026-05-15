package db

import (
	"log"
	"os"

	"github.com/SAYAN02-DEV/iron-bench/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	var err error
	DB, err = gorm.Open(postgres.Open(connStr), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect:", err)
	}

	DB.AutoMigrate(&models.User{})

	log.Println("Connected to database")
}
