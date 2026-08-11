package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment         string
	Port                string
	DatabasePath        string
	ResendAPIKey        string
	ResendWebhookSecret string

	AdminOrigins      []string
	AdminSecureCookie bool
}

func Load() (Config, error) {
	_ = godotenv.Load()

	environment := env("APP_ENV", "development")

	adminOrigins, err := loadAdminOrigins()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:         environment,
		Port:                env("PORT", "8080"),
		DatabasePath:        env("DATABASE_PATH", "./data/gateway.db"),
		ResendAPIKey:        strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		ResendWebhookSecret: strings.TrimSpace(os.Getenv("RESEND_WEBHOOK_SECRET")),

		AdminOrigins:      adminOrigins,
		AdminSecureCookie: !strings.EqualFold(environment, "development"),
	}

	if cfg.ResendAPIKey == "" {
		return Config{}, fmt.Errorf("RESEND_API_KEY is required")
	}

	return cfg, nil
}

func loadAdminOrigins() ([]string, error) {
	keys := []string{
		"ADMIN_ORIGIN",
		"ADMIN_DEV_ORIGIN",
	}

	origins := make([]string, 0, len(keys))

	for _, key := range keys {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}

		origin, err := normalizeOrigin(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}

		origins = append(origins, origin)
	}

	return origins, nil
}

func normalizeOrigin(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid origin")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("origin must use http or https")
	}

	if parsed.Host == "" {
		return "", fmt.Errorf("origin must include a host")
	}

	if parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return "", fmt.Errorf("origin must not include credentials, path, query or fragment")
	}

	return parsed.Scheme + "://" + parsed.Host, nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
