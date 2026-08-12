package api

import (
	"net/http"
	"strings"
)

func emailTesterCORS(
	adminOrigin string,
	adminDevOrigin string,
) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{})

	for _, origin := range []string{
		adminOrigin,
		adminDevOrigin,
	} {
		origin = strings.TrimSpace(origin)
		origin = strings.TrimSuffix(origin, "/")

		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// We only want this CORS exception for the public email API.
			if r.URL.Path != "/v1/emails" {
				next.ServeHTTP(w, r)
				return
			}

			origin := strings.TrimSuffix(
				strings.TrimSpace(r.Header.Get("Origin")),
				"/",
			)

			// Normal server-to-server requests don't normally have Origin.
			// They continue working exactly as before.
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			if _, ok := allowed[origin]; !ok {
				// Don't emit CORS headers for unknown browser origins.
				if r.Method == http.MethodOptions {
					http.Error(w, "origin not allowed", http.StatusForbidden)
					return
				}

				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")

			w.Header().Set(
				"Access-Control-Allow-Methods",
				"POST, OPTIONS",
			)

			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Authorization, Content-Type, Idempotency-Key",
			)

			w.Header().Set(
				"Access-Control-Max-Age",
				"600",
			)

			// Browser preflight must terminate here.
			// Do NOT let it reach project authentication.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}