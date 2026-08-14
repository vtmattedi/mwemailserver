package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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
	AdminAuthMode     string

	IdentityInternalBaseURL       string
	IdentityIntrospectionClientID string
	IdentityIntrospectionSecret   string
	IdentityAudience              string
	IdentityHTTPTimeout           time.Duration

	CheckSPHost bool
	SPHost      string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	environment := env("APP_ENV", "development")

	adminOrigins, err := loadAdminOrigins()
	if err != nil {
		return Config{}, err
	}

	checkSPHost, err := envBool("CHECK_SP_HOST", false)
	if err != nil {
		return Config{}, err
	}

	spHost := ""
	if checkSPHost {
		spHost, err = normalizeHost(strings.TrimSpace(os.Getenv("SP_HOST")))
		if err != nil {
			return Config{}, fmt.Errorf("SP_HOST: %w", err)
		}
	}

	authMode := strings.ToLower(env("ADMIN_AUTH_MODE", "local"))
	if authMode != "local" && authMode != "hybrid" && authMode != "federated" {
		return Config{}, fmt.Errorf("ADMIN_AUTH_MODE must be local, hybrid or federated")
	}

	timeoutSeconds, err := envPositiveInt("MW_IDENTITY_HTTP_TIMEOUT_SECONDS", 5)
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
		AdminAuthMode:     authMode,

		IdentityInternalBaseURL:       strings.TrimSpace(os.Getenv("MW_IDENTITY_INTERNAL_BASE_URL")),
		IdentityIntrospectionClientID: strings.TrimSpace(os.Getenv("MW_IDENTITY_INTROSPECTION_CLIENT_ID")),
		IdentityIntrospectionSecret:   strings.TrimSpace(os.Getenv("MW_IDENTITY_INTROSPECTION_CLIENT_SECRET")),
		IdentityAudience:              env("MW_IDENTITY_AUDIENCE", "mw-mail"),
		IdentityHTTPTimeout:           time.Duration(timeoutSeconds) * time.Second,

		CheckSPHost: checkSPHost,
		SPHost:      spHost,
	}

	if cfg.ResendAPIKey == "" {
		return Config{}, fmt.Errorf("RESEND_API_KEY is required")
	}

	if authMode != "local" {
		if cfg.IdentityInternalBaseURL == "" || cfg.IdentityIntrospectionClientID == "" || cfg.IdentityIntrospectionSecret == "" {
			return Config{}, fmt.Errorf("MW Identity introspection configuration is required in %s admin auth mode", authMode)
		}
		parsed, err := url.Parse(cfg.IdentityInternalBaseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return Config{}, fmt.Errorf("MW_IDENTITY_INTERNAL_BASE_URL must be an absolute HTTP(S) URL")
		}
	}

	return cfg, nil
}

func envPositiveInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
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
		return "", fmt.Errorf(
			"origin must not include credentials, path, query or fragment",
		)
	}

	return parsed.Scheme + "://" + parsed.Host, nil
}

func normalizeHost(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "", fmt.Errorf("host is required when CHECK_SP_HOST=true")
	}

	if strings.Contains(raw, "://") ||
		strings.ContainsAny(raw, "/?#@") {
		return "", fmt.Errorf(
			"host must be a hostname only, without scheme, path, query or fragment",
		)
	}

	host := raw
	if parsedHost, _, err := net.SplitHostPort(raw); err == nil {
		host = parsedHost
	}

	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" {
		return "", fmt.Errorf("invalid host")
	}

	return host, nil
}

func envBool(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}

	return value, nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
