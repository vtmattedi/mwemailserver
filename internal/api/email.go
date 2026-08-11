package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vtmattedi/mw-email-server/internal/auth"
	"github.com/vtmattedi/mw-email-server/internal/email"
)

const maxEmailRequestBody = 1024 * 1024

type sendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
}

type sendEmailResponse struct {
	ID       string `json:"id"`
	Replayed bool   `json:"replayed"`
}

func SendEmail(service *email.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project, ok := auth.ProjectFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if idempotencyKey == "" {
			writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
			return
		}

		if len(idempotencyKey) > email.MaxIdempotencyKeyLength {
			writeError(w, http.StatusBadRequest, "Idempotency-Key must be at most 256 characters")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxEmailRequestBody)

		var req sendEmailRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.From = strings.TrimSpace(req.From)
		req.Subject = strings.TrimSpace(req.Subject)

		for i := range req.To {
			req.To[i] = strings.TrimSpace(req.To[i])
		}

		if req.From == "" || len(req.To) == 0 || req.Subject == "" {
			writeError(w, http.StatusBadRequest, "from, to and subject are required")
			return
		}

		for _, recipient := range req.To {
			if recipient == "" {
				writeError(w, http.StatusBadRequest, "to cannot contain an empty recipient")
				return
			}
		}

		if strings.TrimSpace(req.HTML) == "" && strings.TrimSpace(req.Text) == "" {
			writeError(w, http.StatusBadRequest, "html or text body is required")
			return
		}

		result, err := service.Send(
			r.Context(),
			project.ID,
			idempotencyKey,
			email.Message{
				From:    req.From,
				To:      req.To,
				Subject: req.Subject,
				HTML:    req.HTML,
				Text:    req.Text,
			},
		)
		if err != nil {
			switch {
			case errors.Is(err, email.ErrIdempotencyConflict):
				writeError(w, http.StatusConflict, err.Error())
			case errors.Is(err, email.ErrIdempotencyWindowExpired):
				writeError(w, http.StatusConflict, err.Error())
			default:
				slog.Error(
					"email send failed",
					"project_id", project.ID,
					"error", err,
				)
				writeError(w, http.StatusBadGateway, "email provider request failed")
			}
			return
		}

		if result.Replayed {
			w.Header().Set("Idempotency-Replayed", "true")
		}

		writeJSON(w, http.StatusOK, sendEmailResponse{
			ID:       result.ProviderID,
			Replayed: result.Replayed,
		})
	}
}
