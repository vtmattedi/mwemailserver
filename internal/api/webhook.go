package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/providers/resend"
	"github.com/vtmattedi/mw-email-server/internal/store"
)

const maxWebhookBody = 1024 * 1024

type resendWebhookPayload struct {
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
	Data      struct {
		EmailID string `json:"email_id"`
	} `json:"data"`
}

func ResendWebhook(
	s store.Store,
	verifier *resend.WebhookVerifier,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if verifier == nil {
			writeError(
				w,
				http.StatusServiceUnavailable,
				"Resend webhook verification is not configured",
			)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBody)

		payload, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid webhook body")
			return
		}

		if err := verifier.Verify(payload, r.Header); err != nil {
			writeError(w, http.StatusBadRequest, "invalid webhook signature")
			return
		}

		eventID := strings.TrimSpace(r.Header.Get("svix-id"))
		if eventID == "" {
			writeError(w, http.StatusBadRequest, "missing svix-id")
			return
		}

		var event resendWebhookPayload
		if err := json.Unmarshal(payload, &event); err != nil {
			writeError(w, http.StatusBadRequest, "invalid webhook payload")
			return
		}

		// This endpoint is intentionally scoped to Resend email events.
		// If the Resend endpoint is accidentally subscribed to other event families,
		// acknowledge them without mutating our email tables.
		if !strings.HasPrefix(event.Type, "email.") {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		event.Data.EmailID = strings.TrimSpace(event.Data.EmailID)
		if event.Data.EmailID == "" {
			writeError(w, http.StatusBadRequest, "missing data.email_id")
			return
		}

		occurredAt, err := time.Parse(time.RFC3339Nano, event.CreatedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid created_at")
			return
		}

		duplicate, err := s.ProcessEmailEvent(
			r.Context(),
			store.ProcessEmailEventParams{
				Provider:        "resend",
				ProviderEventID: eventID,
				ProviderEmailID: event.Data.EmailID,
				EventType:       event.Type,
				OccurredAt:      occurredAt.UTC(),
				DeliveryStatus:  resend.DeliveryStatusForEvent(event.Type),
			},
		)
		if err != nil {
			slog.Error(
				"failed to process resend webhook",
				"event_id", eventID,
				"event_type", event.Type,
				"provider_email_id", event.Data.EmailID,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "failed to process webhook")
			return
		}

		if duplicate {
			slog.Debug("ignored duplicate resend webhook", "event_id", eventID)
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
