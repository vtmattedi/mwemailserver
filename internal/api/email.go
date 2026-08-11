package api

import (
	"encoding/json"
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
	ID string `json:"id"`
}

func SendEmail(service *email.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project, ok := auth.ProjectFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
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

		if req.From == "" || len(req.To) == 0 || req.Subject == "" {
			writeError(w, http.StatusBadRequest, "from, to and subject are required")
			return
		}

		if strings.TrimSpace(req.HTML) == "" && strings.TrimSpace(req.Text) == "" {
			writeError(w, http.StatusBadRequest, "html or text body is required")
			return
		}

		result, err := service.Send(r.Context(), project.ID, email.Message{
			From:    req.From,
			To:      req.To,
			Subject: req.Subject,
			HTML:    req.HTML,
			Text:    req.Text,
		})
		if err != nil {
			slog.Error(
				"email send failed",
				"project_id", project.ID,
				"error", err,
			)
			writeError(w, http.StatusBadGateway, "email provider rejected the request")
			return
		}

		writeJSON(w, http.StatusOK, sendEmailResponse{
			ID: result.ProviderID,
		})
	}
}
