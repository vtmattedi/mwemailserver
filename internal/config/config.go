package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment  string
	Port         string
	DatabasePath string
	ResendAPIKey string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Environment:  env("APP_ENV", "development"),
		Port:         env("PORT", "8080"),
		DatabasePath: env("DATABASE_PATH", "./data/gateway.db"),
		ResendAPIKey: strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
	}

	if cfg.ResendAPIKey == "" {
		return Config{}, fmt.Errorf("RESEND_API_KEY is required")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
