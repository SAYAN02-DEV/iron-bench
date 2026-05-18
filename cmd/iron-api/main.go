package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SAYAN02-DEV/iron-bench/internal/config"
	"github.com/SAYAN02-DEV/iron-bench/internal/db"
	"github.com/SAYAN02-DEV/iron-bench/internal/handler/user"
	"github.com/SAYAN02-DEV/iron-bench/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Println(".env not found; falling back to environment variables")
	}

	db.Connect(cfg)

	router := http.NewServeMux()

	router.HandleFunc("GET /test/iron", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("iron-api OK"))
	})

	router.HandleFunc("POST /api/user/signup", user.Signup())
	router.HandleFunc("POST /api/user/signin", user.Signin())
	router.Handle("POST /api/user/upload-url", middleware.RequireAuth(http.HandlerFunc(user.GetUploadURL())))

	addr := ":" + cfg.Port
	srv := http.Server{
		Addr:    addr,
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error:", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Println("server shutdown error:", err)
	}

	db.Close()
}
