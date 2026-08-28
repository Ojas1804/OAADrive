// Package main is the entrypoint for the OAADrive API server. It loads
// config from the environment, connects to Postgres and SeaweedFS, and
// serves the REST API defined in internal/api (see docs/API_CONTRACT.md).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ojas1804/OAADrive/internal/api"
	"github.com/Ojas1804/OAADrive/internal/db"
	"github.com/Ojas1804/OAADrive/internal/storage"
)

type config struct {
	Port               string
	DatabaseURL        string
	SeaweedfsEndpoint  string
	SeaweedfsAccessKey string
	SeaweedfsSecretKey string
	JWTSecret          string
}

func loadConfig() config {
	return config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		SeaweedfsEndpoint:  getEnv("SEAWEEDFS_S3_ENDPOINT", ""),
		SeaweedfsAccessKey: getEnv("SEAWEEDFS_ACCESS_KEY", ""),
		SeaweedfsSecretKey: getEnv("SEAWEEDFS_SECRET_KEY", ""),
		JWTSecret:          getEnv("JWT_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg := loadConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(connectCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	storageClient, err := storage.NewClient(cfg.SeaweedfsEndpoint, cfg.SeaweedfsAccessKey, cfg.SeaweedfsSecretKey)
	if err != nil {
		log.Fatalf("storage client init failed: %v", err)
	}

	server := api.NewServer(
		db.NewUserStore(pool),
		db.NewFileStore(pool),
		db.NewAuditStore(pool),
		storageClient,
		cfg.JWTSecret,
	)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.Routes(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("OAADrive API listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	log.Println("server stopped")
}
