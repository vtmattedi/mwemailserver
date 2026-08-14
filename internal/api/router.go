package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/admin"
	"github.com/vtmattedi/mw-email-server/internal/auth"
	"github.com/vtmattedi/mw-email-server/internal/email"
	"github.com/vtmattedi/mw-email-server/internal/federation"
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
	AdminAuthMode     string
	FederationClient  federation.Introspector

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
	mode := deps.AdminAuthMode
	if mode == "" {
		mode = "local"
	}
	concealCfg := admin.ConcealmentConfig{
		AllowedOrigins: deps.AdminOrigins,
		CheckHost:      deps.CheckSPHost,
		RequiredHost:   deps.SPHost,
	}
	localEndpoint := func(handler http.Handler) http.Handler {
		if mode == "federated" {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeError(w, http.StatusUnauthorized, "local admin authentication is disabled")
			})
		}
		return admin.Conceal(concealCfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			handler.ServeHTTP(w, r)
		}))
	}

	mux.Handle(
		"POST /admin/api/auth/login",
		localEndpoint(AdminLogin(deps.AdminService, deps.AdminSecureCookie)),
	)

	authorize := func(scope string, mutation bool, handler http.Handler) http.Handler {
		handler = auditAdminOperation(scope, handler)
		local := admin.RequireSession(
			deps.AdminService,
			handler,
		)
		if mutation {
			local = admin.RequireSession(deps.AdminService, admin.RequireCSRF(deps.AdminService, handler))
		}
		local = admin.Conceal(concealCfg, local)
		return adminAuthRoute(mode, deps.FederationClient, concealCfg, scope, local, handler)
	}

	mux.Handle(
		"GET /admin/api/auth/me",
		localEndpoint(admin.RequireSession(deps.AdminService,
			http.HandlerFunc(AdminMe),
		)),
	)

	mux.Handle(
		"POST /admin/api/auth/csrf",
		localEndpoint(admin.RequireSession(deps.AdminService,
			http.HandlerFunc(
				AdminRotateCSRF(deps.AdminService),
			),
		)),
	)

	mux.Handle(
		"POST /admin/api/auth/logout",
		localEndpoint(admin.RequireSession(deps.AdminService, admin.RequireCSRF(deps.AdminService,
			http.HandlerFunc(
				AdminLogout(
					deps.AdminService,
					deps.AdminSecureCookie,
				),
			),
		))),
	)

	mux.Handle(
		"GET /admin/api/health",
		authorize("mail.projects.read", false,
			http.HandlerFunc(AdminHealth),
		),
	)

	mux.Handle(
		"GET /admin/api/projects",
		authorize("mail.projects.read", false,
			http.HandlerFunc(
				AdminListProjects(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"POST /admin/api/projects",
		authorize("mail.projects.write", true,
			http.HandlerFunc(
				AdminCreateProject(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"PATCH /admin/api/projects/{id}",
		authorize("mail.projects.write", true,
			http.HandlerFunc(
				AdminUpdateProject(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"POST /admin/api/projects/{id}/rotate-key",
		authorize("mail.credentials.rotate", true,
			http.HandlerFunc(
				AdminRotateProjectKey(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"GET /admin/api/projects/{id}/request-domains",
		authorize("mail.projects.read", false, http.HandlerFunc(
			AdminGetProjectRequestDomains(deps.Store),
		)),
	)
	mux.Handle(
		"PUT /admin/api/projects/{id}/request-domains",
		authorize("mail.projects.write", true, http.HandlerFunc(
			AdminPutProjectRequestDomains(deps.Store),
		)),
	)

	mux.Handle(
		"GET /admin/api/emails",
		authorize("mail.emails.read", false,
			http.HandlerFunc(
				AdminListEmails(deps.AdminService),
			),
		),
	)

	mux.Handle(
		"GET /admin/api/emails/by-project",
		authorize("mail.usage.read", false,
			http.HandlerFunc(
				AdminProjectEmailCounts(deps.Store),
			),
		),
	)

	mux.Handle(
		"GET /admin/api/emails/{id}",
		authorize("mail.emails.read", false,
			http.HandlerFunc(
				AdminGetEmail(deps.AdminService),
			),
		),
	)

	return mux
}

func adminAuthRoute(mode string, client federation.Introspector, concealCfg admin.ConcealmentConfig, scope string, local, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
			if mode == "local" || client == nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			federated := federation.Authenticate(client, scope, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fp, _ := federation.PrincipalFromContext(r.Context())
				authTime := fp.AuthTime
				principal := admin.Principal{Source: "federated", IdentitySubject: &fp.Subject, Scopes: fp.Scopes, AMR: fp.AMR, Audience: fp.Audience}
				if !authTime.IsZero() {
					principal.AuthTime = &authTime
				}
				handler.ServeHTTP(w, r.WithContext(admin.WithPrincipal(r.Context(), principal)))
			}))
			admin.RequireHost(concealCfg, federated).ServeHTTP(w, r)
			return
		}
		if mode == "federated" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		local.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func auditAdminOperation(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		principal, ok := admin.PrincipalFromContext(r.Context())
		if !ok {
			return
		}
		attrs := []any{"actor_type", principal.Source, "method", r.Method, "path", r.URL.Path, "status", recorder.status, "authorized_scope", scope, "request_id", r.Header.Get("X-Request-ID")}
		if principal.IdentitySubject != nil {
			attrs = append(attrs, "actor_identity_sub", *principal.IdentitySubject, "delegated_audience", principal.Audience, "authorized_scopes", strings.Join(principal.Scopes, " "))
		}
		if principal.AuthTime != nil {
			attrs = append(attrs, "actor_auth_time", principal.AuthTime.Format(time.RFC3339))
		}
		if len(principal.AMR) > 0 {
			attrs = append(attrs, "actor_amr", strings.Join(principal.AMR, " "))
		}
		slog.Info("admin operation", attrs...)
	})
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
