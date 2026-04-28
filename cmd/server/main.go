package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"agentbackend/internal/config"
	"agentbackend/internal/httpserver"
	"agentbackend/internal/storage"
)

func main() {
	cfg := config.Load()

	db, err := storage.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := storage.RunMigrations(ctx, db, "migrations"); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	router := httpserver.NewRouter(db)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	log.Printf("starting server on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}

