package federation

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey uint8

const principalContextKey contextKey = 0

func Authenticate(client Introspector, requiredScope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			logRejection(r, "federation.authorization_malformed")
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		principal, err := client.Introspect(r.Context(), token)
		if err != nil {
			if errors.Is(err, ErrAudienceMismatch) {
				logRejection(r, "federation.audience_mismatch")
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			if errors.Is(err, ErrInvalidToken) {
				logRejection(r, "federation.token_inactive")
			} else {
				logRejection(r, "federation.introspection_failed")
			}
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !principal.HasScope(requiredScope) {
			logRejection(r, "federation.scope_missing")
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func logRejection(r *http.Request, reason string) {
	slog.Warn("federated authentication rejected", "reason", reason, "request_id", r.Header.Get("X-Request-ID"), "path", r.URL.Path)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	returnToken := ""
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		returnToken = parts[1]
	}
	return returnToken, returnToken != ""
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}` + "\n"))
}
