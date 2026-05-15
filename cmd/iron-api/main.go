package main

import (
	"log"
	"net/http"

	"github.com/SAYAN02-DEV/iron-bench/internal/config"
	"github.com/SAYAN02-DEV/iron-bench/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Println(".env not found; falling back to environment variables")
	}

	db.Connect(cfg)

	router := http.NewServeMux()

	router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	addr := ":" + cfg.Port
	srv := http.Server{
		Addr: addr,
		Handler: router,
	}

	log.Printf("Starting server on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
