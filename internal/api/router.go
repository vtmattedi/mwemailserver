package api

import (
	"encoding/json"
	"net/http"

	"github.com/vtmattedi/mw-email-server/internal/auth"
	"github.com/vtmattedi/mw-email-server/internal/email"
	"github.com/vtmattedi/mw-email-server/internal/store"
)

type Dependencies struct {
	EmailService *email.Service
	Store        store.Store
}

func NewRouter(deps Dependencies) http.Handler {
	root := http.NewServeMux()
	root.HandleFunc("GET /health", Health)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /v1/emails", SendEmail(deps.EmailService))

	root.Handle("/v1/", auth.Middleware(deps.Store)(protected))

	return root
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if value != nil {
		json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
