package federation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc, timeout time.Duration) (*Client, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	client, err := NewClient(server.URL, "resource-id", "resource-secret", "mw-mail", timeout)
	if err != nil {
		t.Fatal(err)
	}
	return client, server.Close
}

func TestClientIntrospectValidToken(t *testing.T) {
	client, closeServer := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "resource-id" || password != "resource-secret" {
			t.Error("missing Basic credentials")
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("token") != "opaque-token" {
			t.Error("missing token form value")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active":true,"sub":"usr_123","aud":["other","mw-mail"],"scope":"mail.projects.read mail.emails.read","auth_time":1700000000,"amr":["pwd","webauthn"]}`))
	}, time.Second)
	defer closeServer()

	principal, err := client.Introspect(context.Background(), "opaque-token")
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "usr_123" || principal.Audience != "mw-mail" {
		t.Fatalf("unexpected principal: %+v", principal)
	}
	if !principal.HasScope("mail.projects.read") || !principal.HasScope("mail.emails.read") {
		t.Fatalf("missing scopes: %v", principal.Scopes)
	}
	if principal.AuthTime.Unix() != 1700000000 || strings.Join(principal.AMR, " ") != "pwd webauthn" {
		t.Fatalf("authentication context lost: %+v", principal)
	}
}

func TestClientIntrospectionRejections(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"inactive", 200, `{"active":false}`, ErrInvalidToken},
		{"empty subject", 200, `{"active":true,"sub":"","aud":"mw-mail"}`, ErrInvalidToken},
		{"wrong audience", 200, `{"active":true,"sub":"usr_1","aud":"mw-oauth"}`, ErrAudienceMismatch},
		{"identity 401", 401, `{}`, nil}, {"identity 403", 403, `{}`, nil}, {"identity 500", 500, `{}`, nil},
		{"invalid JSON", 200, `{`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, closeServer := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}, time.Second)
			defer closeServer()
			_, err := client.Introspect(context.Background(), "token")
			if err == nil {
				t.Fatal("expected rejection")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestClientRejectsOversizedResponse(t *testing.T) {
	client, closeServer := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("x", maxIntrospectionBody+1))
	}, time.Second)
	defer closeServer()
	_, err := client.Introspect(context.Background(), "token")
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("got %v", err)
	}
}

func TestClientTimesOut(t *testing.T) {
	client, closeServer := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"active":true}`))
	}, 5*time.Millisecond)
	defer closeServer()
	_, err := client.Introspect(context.Background(), "token")
	if err == nil {
		t.Fatal("expected timeout")
	}
}
