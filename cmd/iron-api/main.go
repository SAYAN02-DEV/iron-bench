package main

import (
	"context"
	"fmt"
	"log"

	"github.com/SAYAN02-DEV/iron-bench/db"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	db.Connect()
	defer db.Conn.Close(context.Background())
	fmt.Println("Connected to database")
}
