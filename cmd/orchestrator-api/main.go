package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/SAYAN02-DEV/iron-bench/internal/config"
	"github.com/SAYAN02-DEV/iron-bench/internal/db"
	"github.com/SAYAN02-DEV/iron-bench/internal/handler/orchestrator"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Println("Failed to load config:", err)
	}

	db.Connect(cfg)

	router := http.NewServeMux()

	router.HandleFunc("GET /test/orchestrator", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Orchestrater OK"))
	})

	router.HandleFunc("POST /file/download", orchestrator.DownloadFilesHandler())

	addr := ":" + cfg.OrchestratorPort
	srv := http.Server{
		Addr:    addr,
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error:", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Println("Failed to shutdown server:", err)
	}

	db.Close()
}
