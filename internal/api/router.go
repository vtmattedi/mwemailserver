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

// MW_ALLOWED_REQUEST_DOMAINS_V1
// MW_PROJECT_EMAIL_COUNTS_V1
type GatewayStore interface {
	store.Store
	RequestDomainStore
	ProjectEmailCountStore
}

type Dependencies struct {
	EmailService          *email.Service
	Store                 GatewayStore
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

	/*
		Project-authenticated public API.

		The actual POST remains protected by project Bearer authentication.

		CORS is applied around the root mux below so that browser OPTIONS
		preflight requests can be handled before they reach this method-specific
		route or the project auth middleware.
	*/
	protected := http.NewServeMux()

	protected.Handle(
		"POST /v1/emails",
		RequireAllowedRequestDomain(
			deps.Store,
			http.HandlerFunc(SendEmail(deps.EmailService)),
		),
	)

	root.Handle(
		"/v1/",
		auth.Middleware(deps.Store)(protected),
	)

	/*
		Admin API.

		Admin concealment/session/CSRF remain completely separate from the
		/v1/emails tester CORS behavior.
	*/
	adminHandler := newAdminRouter(deps)

	root.Handle("/admin/api", adminHandler)
	root.Handle("/admin/api/", adminHandler)

	/*
		AdminOrigins currently contains:

			[0] production admin origin
			[1] optional development admin origin

		These are reused to allow the protected admin SPA to behave like an
		external client and call POST /v1/emails with a project API key.

		emailTesterCORS itself only acts on /v1/emails.
	*/
	var adminOrigin string
	var adminDevOrigin string

	if len(deps.AdminOrigins) > 0 {
		adminOrigin = deps.AdminOrigins[0]
	}

	if len(deps.AdminOrigins) > 1 {
		adminDevOrigin = deps.AdminOrigins[1]
	}

	return RequestEmailCORS(emailTesterCORS(
		adminOrigin,
		adminDevOrigin,
	)(root))
}

func newAdminRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"POST /admin/api/auth/login",
		AdminLogin(deps.AdminService, deps.AdminSecureCookie),
	)

	sessionOnly := func(handler http.Handler) http.Handler {
		return admin.RequireSession(
			deps.AdminService,
			handler,
		)
	}

	sessionAndCSRF := func(handler http.Handler) http.Handler {
		return admin.RequireSession(
			deps.AdminService,
			admin.RequireCSRF(
				deps.AdminService,
				handler,
			),
		)
	}

	mux.Handle(
		"GET /admin/api/auth/me",
		sessionOnly(
			http.HandlerFunc(AdminMe),
		),
	)

	mux.Handle(
		"POST /admin/api/auth/csrf",
		sessionOnly(
			http.HandlerFunc(
				AdminRotateCSRF(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"POST /admin/api/auth/logout",
		sessionAndCSRF(
			http.HandlerFunc(
				AdminLogout(
					deps.AdminService,
					deps.AdminSecureCookie,
				),
			),
		),
	)

	mux.Handle(
		"GET /admin/api/health",
		sessionOnly(
			http.HandlerFunc(AdminHealth),
		),
	)

	mux.Handle(
		"GET /admin/api/projects",
		sessionOnly(
			http.HandlerFunc(
				AdminListProjects(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"POST /admin/api/projects",
		sessionAndCSRF(
			http.HandlerFunc(
				AdminCreateProject(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"PATCH /admin/api/projects/{id}",
		sessionAndCSRF(
			http.HandlerFunc(
				AdminUpdateProject(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"POST /admin/api/projects/{id}/rotate-key",
		sessionAndCSRF(
			http.HandlerFunc(
				AdminRotateProjectKey(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"GET /admin/api/projects/{id}/request-domains",
		sessionOnly(http.HandlerFunc(
			AdminGetProjectRequestDomains(deps.Store),
		)),
	)
	mux.Handle(
		"PUT /admin/api/projects/{id}/request-domains",
		sessionAndCSRF(http.HandlerFunc(
			AdminPutProjectRequestDomains(deps.Store),
		)),
	)

	mux.Handle(
		"GET /admin/api/emails",
		sessionOnly(
			http.HandlerFunc(
				AdminListEmails(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"GET /admin/api/emails/by-project",
		sessionOnly(
			http.HandlerFunc(
				AdminProjectEmailCounts(deps.Store),
			),
		),
	)

	mux.Handle(
		"GET /admin/api/emails/{id}",
		sessionOnly(
			http.HandlerFunc(
				AdminGetEmail(deps.AdminService),
			),
		),
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
	writeJSON(
		w,
		status,
		map[string]string{
			"error": message,
		},
	)
}
