package admin

import (
	"context"
	"net/http"
	"strings"
)

const SessionCookieName = "mw_admin_session"

type contextKey uint8

const sessionContextKey contextKey = iota

func OriginGate(
	allowedOrigins []string,
	next http.Handler,
) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))

		if _, ok := allowed[origin]; !ok {
			// Intentionally indistinguishable from an ordinary unhandled route.
			http.NotFound(w, r)
			return
		}

		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, X-CSRF-Token",
		)
		w.Header().Set(
			"Access-Control-Allow-Methods",
			"GET, POST, PATCH, DELETE, OPTIONS",
		)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequireSession(
	service *Service,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeUnauthorized(w)
			return
		}

		session, err := service.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeUnauthorized(w)
			return
		}

		ctx := context.WithValue(r.Context(), sessionContextKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireCSRF(
	service *Service,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		session, ok := SessionFromContext(r.Context())
		if !ok {
			writeUnauthorized(w)
			return
		}

		if !service.VerifyCSRF(
			session,
			strings.TrimSpace(r.Header.Get("X-CSRF-Token")),
		) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func SessionFromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionContextKey).(Session)
	return session, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
