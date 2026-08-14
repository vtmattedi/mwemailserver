package config

import (
	"strings"
	"testing"
)

func setMinimumEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("ADMIN_ORIGIN", "")
	t.Setenv("ADMIN_DEV_ORIGIN", "")
	t.Setenv("CHECK_SP_HOST", "false")
}

func TestLoadAdminAuthModes(t *testing.T) {
	t.Run("local is the compatibility default", func(t *testing.T) {
		setMinimumEnvironment(t)
		t.Setenv("ADMIN_AUTH_MODE", "")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AdminAuthMode != "local" {
			t.Fatalf("got %q", cfg.AdminAuthMode)
		}
	})

	t.Run("hybrid requires Identity credentials", func(t *testing.T) {
		setMinimumEnvironment(t)
		t.Setenv("ADMIN_AUTH_MODE", "hybrid")
		t.Setenv("MW_IDENTITY_INTERNAL_BASE_URL", "")
		t.Setenv("MW_IDENTITY_INTROSPECTION_CLIENT_ID", "")
		t.Setenv("MW_IDENTITY_INTROSPECTION_CLIENT_SECRET", "")
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "introspection configuration") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("federated configuration", func(t *testing.T) {
		setMinimumEnvironment(t)
		t.Setenv("ADMIN_AUTH_MODE", "federated")
		t.Setenv("MW_IDENTITY_INTERNAL_BASE_URL", "http://identity.internal:7100")
		t.Setenv("MW_IDENTITY_INTROSPECTION_CLIENT_ID", "mail-resource")
		t.Setenv("MW_IDENTITY_INTROSPECTION_CLIENT_SECRET", "secret")
		t.Setenv("MW_IDENTITY_AUDIENCE", "mw-mail")
		t.Setenv("MW_IDENTITY_HTTP_TIMEOUT_SECONDS", "7")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AdminAuthMode != "federated" || cfg.IdentityHTTPTimeout.Seconds() != 7 {
			t.Fatalf("unexpected config: %+v", cfg)
		}
	})
}
