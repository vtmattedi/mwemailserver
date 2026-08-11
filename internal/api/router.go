package api

import (
	"encoding/json"
	"net/http"

	"github.com/vtmattedi/mw-email-server/internal/admin"
	"github.com/vtmattedi/mw-email-server/internal/auth"
	"github.com/vtmattedi/mw-email-server/internal/email"
	"github.com/vtmattedi/mw-email-server/internal/providers/resend"
	"github.com/vtmattedi/mw-email-server/internal/store"
)

type Dependencies struct {
	EmailService          *email.Service
	Store                 store.Store
	ResendWebhookVerifier *resend.WebhookVerifier

	AdminService      *admin.Service
	AdminOrigins      []string
	AdminSecureCookie bool

	CheckSPHost bool
	SPHost      string
}

func NewRouter(deps Dependencies) http.Handler {
	root := http.NewServeMux()

	root.HandleFunc("GET /health", Health)
	root.HandleFunc(
		"POST /webhooks/resend",
		ResendWebhook(deps.Store, deps.ResendWebhookVerifier),
	)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /v1/emails", SendEmail(deps.EmailService))
	root.Handle("/v1/", auth.Middleware(deps.Store)(protected))

	adminHandler := newAdminRouter(deps)
	root.Handle("/admin/api", adminHandler)
	root.Handle("/admin/api/", adminHandler)

	return root
}

func newAdminRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"POST /admin/api/auth/login",
		AdminLogin(deps.AdminService, deps.AdminSecureCookie),
	)

	sessionOnly := func(handler http.Handler) http.Handler {
		return admin.RequireSession(deps.AdminService, handler)
	}

	sessionAndCSRF := func(handler http.Handler) http.Handler {
		return admin.RequireSession(
			deps.AdminService,
			admin.RequireCSRF(deps.AdminService, handler),
		)
	}

	mux.Handle(
		"GET /admin/api/auth/me",
		sessionOnly(http.HandlerFunc(AdminMe)),
	)
	mux.Handle(
		"POST /admin/api/auth/csrf",
		sessionOnly(http.HandlerFunc(AdminRotateCSRF(deps.AdminService))),
	)
	mux.Handle(
		"POST /admin/api/auth/logout",
		sessionAndCSRF(http.HandlerFunc(
			AdminLogout(deps.AdminService, deps.AdminSecureCookie),
		)),
	)

	mux.Handle(
		"GET /admin/api/health",
		sessionOnly(http.HandlerFunc(AdminHealth)),
	)

	mux.Handle(
		"GET /admin/api/projects",
		sessionOnly(http.HandlerFunc(AdminListProjects(deps.AdminService))),
	)
	mux.Handle(
		"POST /admin/api/projects",
		sessionAndCSRF(http.HandlerFunc(AdminCreateProject(deps.AdminService))),
	)
	mux.Handle(
		"PATCH /admin/api/projects/{id}",
		sessionAndCSRF(http.HandlerFunc(AdminUpdateProject(deps.AdminService))),
	)
	mux.Handle(
		"POST /admin/api/projects/{id}/rotate-key",
		sessionAndCSRF(http.HandlerFunc(AdminRotateProjectKey(deps.AdminService))),
	)

	mux.Handle(
		"GET /admin/api/emails",
		sessionOnly(http.HandlerFunc(AdminListEmails(deps.AdminService))),
	)
	mux.Handle(
		"GET /admin/api/emails/{id}",
		sessionOnly(http.HandlerFunc(AdminGetEmail(deps.AdminService))),
	)

	return admin.Conceal(
		admin.ConcealmentConfig{
			AllowedOrigins: deps.AdminOrigins,
			CheckHost:      deps.CheckSPHost,
			RequiredHost:   deps.SPHost,
		},
		mux,
	)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
