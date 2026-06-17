package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort      string
	DatabaseURL     string
	JWTSecret       string
	MinIOEndpoint   string
	MinIOAccessKey  string
	MinIOSecretKey  string
	MinIOUseTLS     bool
	AccessTokenTTL  int
	RefreshTokenTTL int
}

func Load() *Config {
	return &Config{
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		DatabaseURL:     mustEnv("DATABASE_URL"),
		JWTSecret:       mustEnv("JWT_SECRET"),
		MinIOEndpoint:   mustEnv("MINIO_ENDPOINT"),
		MinIOAccessKey:  mustEnv("MINIO_ACCESS_KEY"),
		MinIOSecretKey:  mustEnv("MINIO_SECRET_KEY"),
		MinIOUseTLS:     getBoolEnv("MINIO_USE_TLS", false),
		AccessTokenTTL:  getIntEnv("ACCESS_TOKEN_TTL_MINUTES", 15),
		RefreshTokenTTL: getIntEnv("REFRESH_TOKEN_TTL_DAYS", 30),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing required environment variable: " + key)
	}
	return v
}

func getBoolEnv(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getIntEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}
