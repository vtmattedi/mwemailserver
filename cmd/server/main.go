package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/api"
	"github.com/vtmattedi/mw-email-server/internal/config"
	"github.com/vtmattedi/mw-email-server/internal/email"
	"github.com/vtmattedi/mw-email-server/internal/providers/resend"
	"github.com/vtmattedi/mw-email-server/internal/store/sqlite"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := sqlite.Open(cfg.DatabasePath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	provider := resend.New(cfg.ResendAPIKey)
	emailService := email.NewService(provider, db)

	var resendWebhookVerifier *resend.WebhookVerifier
	if cfg.ResendWebhookSecret != "" {
		resendWebhookVerifier, err = resend.NewWebhookVerifier(cfg.ResendWebhookSecret)
		if err != nil {
			slog.Error("invalid RESEND_WEBHOOK_SECRET", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Warn(
			"RESEND_WEBHOOK_SECRET is empty; /webhooks/resend will return 503 until configured",
		)
	}

	router := api.NewRouter(api.Dependencies{
		EmailService:          emailService,
		Store:                 db,
		ResendWebhookVerifier: resendWebhookVerifier,
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("MW Email Server listening", "addr", server.Addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
