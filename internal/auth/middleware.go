package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/vtmattedi/mw-email-server/internal/domain"
	"github.com/vtmattedi/mw-email-server/internal/store"
)

type contextKey uint8

const projectContextKey contextKey = iota

func Middleware(s store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			token, ok := strings.CutPrefix(header, "Bearer ")
			token = strings.TrimSpace(token)

			if !ok || token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			project, err := s.GetProjectByTokenHash(r.Context(), HashToken(token))
			if err != nil || project == nil || !project.Enabled {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), projectContextKey, project)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ProjectFromContext(ctx context.Context) (*domain.Project, bool) {
	project, ok := ctx.Value(projectContextKey).(*domain.Project)
	return project, ok && project != nil
}
