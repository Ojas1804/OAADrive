// Package main is the entrypoint for the OAADrive API server.
//
// This is currently a minimal placeholder: it exposes a health-check
// endpoint and logs the configuration it was started with, so the
// Docker/Compose stack has a real service to build and run against
// while the actual API (auth, uploads, files, admin) is implemented
// in internal/api, internal/auth, internal/db, and internal/storage.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type config struct {
	Port string
	DatabaseURL string
	SeaweedfsEndpoint string
	SeaweedfsAccessKey string
	SeaweedfsSecretKey string
	JWTSecret string
}

func loadConfig() config {
	return config{
		Port: getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		SeaweedfsEndpoint: getEnv("SEAWEEDFS_S3_ENDPOINT", ""),
		SeaweedfsAccessKey: getEnv("SEAWEEDFS_ACCESS_KEY", ""),
		SeaweedfsSecretKey: getEnv("SEAWEEDFS_SECRET_KEY", ""),
		JWTSecret: getEnv("JWT_SECRET", ""),
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

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)

	log.Printf("OAADrive API listening on :%s (db configured: %v, seaweedfs configured: %v)",
		cfg.Port, cfg.DatabaseURL != "", cfg.SeaweedfsEndpoint != "")

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: mux,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}
