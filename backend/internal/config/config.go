// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all runtime configuration for the API.
type Config struct {
	DatabaseURL     string
	HTTPAddr        string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Env             string // "development" | "production"
}

// Load reads configuration from environment variables, applying sensible
// development defaults so the app runs out of the box with docker-compose.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:     getenv("DATABASE_URL", "postgres://wepeople:wepeople@localhost:5432/wepeople?sslmode=disable"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		JWTSecret:       getenv("JWT_SECRET", "dev-insecure-secret-change-me"),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		Env:             getenv("ENV", "development"),
	}

	if cfg.Env == "production" && cfg.JWTSecret == "dev-insecure-secret-change-me" {
		return Config{}, fmt.Errorf("JWT_SECRET must be set in production")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
