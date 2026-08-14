package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vtmattedi/mw-email-server/internal/admin"
	"github.com/vtmattedi/mw-email-server/internal/federation"
)

type routeIntrospector struct {
	principal federation.Principal
	err       error
	calls     int
}

func (s *routeIntrospector) Introspect(_ context.Context, _ string) (federation.Principal, error) {
	s.calls++
	return s.principal, s.err
}

func TestAdminAuthenticationModes(t *testing.T) {
	tests := []struct {
		name, mode, header                                 string
		introspectionErr                                   error
		wantStatus, wantLocalCalls, wantIntrospectionCalls int
	}{
		{"local accepts local", "local", "", nil, 204, 1, 0},
		{"local rejects federated", "local", "Bearer valid", nil, 401, 0, 0},
		{"hybrid accepts local", "hybrid", "", nil, 204, 1, 0},
		{"hybrid accepts federated", "hybrid", "Bearer valid", nil, 204, 0, 1},
		{"federated rejects local", "federated", "", nil, 401, 0, 0},
		{"invalid bearer never falls back", "hybrid", "Bearer invalid", errors.New("inactive"), 401, 0, 1},
		{"malformed bearer never falls back", "hybrid", "Bearer one two", nil, 401, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localCalls := 0
			local := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { localCalls++; w.WriteHeader(http.StatusNoContent) })
			business := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				principal, ok := admin.PrincipalFromContext(r.Context())
				if !ok || principal.Source != "federated" {
					t.Error("unified federated principal missing")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			stub := &routeIntrospector{principal: federation.Principal{Subject: "usr_1", Audience: "mw-mail", Scopes: []string{"mail.projects.read"}}, err: tt.introspectionErr}
			handler := adminAuthRoute(tt.mode, stub, admin.ConcealmentConfig{}, "mail.projects.read", local, business)
			req := httptest.NewRequest(http.MethodGet, "/admin/api/projects", nil)
			req.Header.Set("Authorization", tt.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tt.wantStatus || localCalls != tt.wantLocalCalls || stub.calls != tt.wantIntrospectionCalls {
				t.Fatalf("status=%d local=%d introspection=%d; want %d/%d/%d", response.Code, localCalls, stub.calls, tt.wantStatus, tt.wantLocalCalls, tt.wantIntrospectionCalls)
			}
		})
	}
}
