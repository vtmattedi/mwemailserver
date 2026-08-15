package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vtmattedi/mw-email-server/internal/federation"
)

func TestFederatedAPIDocumentationRoutes(t *testing.T) {
	identity := &routeIntrospector{principal: federation.Principal{
		Subject: "usr_docs", Audience: "mw-mail", Scopes: []string{"mail.server.read"},
	}}
	router := NewRouter(Dependencies{
		AdminAuthMode: "federated", FederationClient: identity,
	})

	request := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer delegated-token")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	list := request("/admin/api/docs")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"version":"v1"`) {
		t.Fatalf("list status=%d body=%q", list.Code, list.Body.String())
	}

	document := request("/admin/api/docs/v1")
	if document.Code != http.StatusOK || document.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("document status=%d content-type=%q", document.Code, document.Header().Get("Content-Type"))
	}
	var payload adminDocResponse
	if err := json.Unmarshal(document.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != "v1" || payload.Status != "current" ||
		payload.ContentType != "text/markdown" ||
		!strings.Contains(payload.Documentation, "mail.projects.delete") {
		t.Fatalf("unexpected documentation response: %+v", payload)
	}

	identity.principal.Scopes = []string{"mail.projects.read"}
	if forbidden := request("/admin/api/docs"); forbidden.Code != http.StatusForbidden {
		t.Fatalf("missing server scope status=%d, want 403", forbidden.Code)
	}
}
