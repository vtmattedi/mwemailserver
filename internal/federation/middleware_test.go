package federation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubIntrospector struct {
	principal Principal
	err       error
	calls     int
}

func (s *stubIntrospector) Introspect(_ context.Context, _ string) (Principal, error) {
	s.calls++
	return s.principal, s.err
}

func TestAuthenticateMiddleware(t *testing.T) {
	tests := []struct {
		name, header, scope string
		principal           Principal
		err                 error
		status              int
		calls               int
	}{
		{"valid", "Bearer opaque", "mail.projects.read", Principal{Subject: "usr_1", Scopes: []string{"mail.projects.read", "mail.emails.read"}}, nil, 204, 1},
		{"missing", "", "mail.projects.read", Principal{}, nil, 401, 0},
		{"malformed scheme", "Basic opaque", "mail.projects.read", Principal{}, nil, 401, 0},
		{"malformed extra part", "Bearer one two", "mail.projects.read", Principal{}, nil, 401, 0},
		{"inactive", "Bearer opaque", "mail.projects.read", Principal{}, ErrInvalidToken, 401, 1},
		{"introspection failure", "Bearer opaque", "mail.projects.read", Principal{}, errors.New("unavailable"), 401, 1},
		{"wrong audience", "Bearer opaque", "mail.projects.read", Principal{}, ErrAudienceMismatch, 403, 1},
		{"scope missing", "Bearer opaque", "mail.projects.write", Principal{Subject: "usr_1", Scopes: []string{"mail.projects.read"}}, nil, 403, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubIntrospector{principal: tt.principal, err: tt.err}
			handler := Authenticate(stub, tt.scope, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, ok := PrincipalFromContext(r.Context()); !ok {
					t.Error("principal missing from context")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, "/admin/api/projects", nil)
			req.Header.Set("Authorization", tt.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tt.status || stub.calls != tt.calls {
				t.Fatalf("status=%d calls=%d; want status=%d calls=%d", response.Code, stub.calls, tt.status, tt.calls)
			}
		})
	}
}
