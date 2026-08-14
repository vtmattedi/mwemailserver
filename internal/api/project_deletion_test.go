package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/vtmattedi/mw-email-server/internal/admin"
	projectauth "github.com/vtmattedi/mw-email-server/internal/auth"
	"github.com/vtmattedi/mw-email-server/internal/federation"
	"github.com/vtmattedi/mw-email-server/internal/store"
	"github.com/vtmattedi/mw-email-server/internal/store/sqlite"
)

func TestFederatedProjectDeletionContract(t *testing.T) {
	dbStore, err := sqlite.Open(filepath.Join(t.TempDir(), "delete-api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbStore.Close()

	service := admin.NewService(dbStore, admin.DefaultSessionTTL)
	deletable, err := service.CreateProject(t.Context(), "Delete through API", nil)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := service.CreateProject(t.Context(), "Retained through API", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := dbStore.ReserveEmailAttempt(t.Context(), store.ReserveEmailAttemptParams{
		ProjectID:          retained.Project.ID,
		Provider:           "resend",
		Sender:             "sender@example.com",
		Subject:            "retained",
		RecipientCount:     1,
		IdempotencyKeyHash: "retained-key",
		RequestHash:        "retained-request",
	}); err != nil {
		t.Fatal(err)
	}

	identity := &routeIntrospector{principal: federation.Principal{
		Subject: "usr_control_plane", Audience: "mw-mail", Scopes: []string{"mail.projects.write"},
	}}
	router := NewRouter(Dependencies{
		Store: dbStore, AdminService: service, AdminAuthMode: "federated", FederationClient: identity,
	})

	request := func(projectID int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, "/admin/api/projects/"+formatID(projectID), nil)
		req.Header.Set("Authorization", "Bearer delegated-token")
		req.Header.Set("X-Request-ID", "req-api-delete")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	if response := request(deletable.Project.ID); response.Code != http.StatusForbidden {
		t.Fatalf("write scope deletion status=%d, want 403", response.Code)
	}
	if _, err := dbStore.GetProjectByTokenHash(t.Context(), projectauth.HashToken(deletable.APIKey)); err != nil {
		t.Fatalf("scope rejection changed project: %v", err)
	}

	identity.principal.Scopes = []string{"mail.projects.delete"}
	if response := request(deletable.Project.ID); response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("deletion status=%d body=%q, want empty 204", response.Code, response.Body.String())
	}
	if _, err := dbStore.GetProjectByTokenHash(t.Context(), projectauth.HashToken(deletable.APIKey)); err != sql.ErrNoRows {
		t.Fatalf("deleted API credential lookup returned %v, want sql.ErrNoRows", err)
	}
	if response := request(deletable.Project.ID); response.Code != http.StatusNotFound {
		t.Fatalf("repeated deletion status=%d, want 404", response.Code)
	}

	response := request(retained.Project.ID)
	if response.Code != http.StatusConflict {
		t.Fatalf("retained deletion status=%d body=%q, want 409", response.Code, response.Body.String())
	}
	var problem map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem["code"] != "project_has_dependencies" {
		t.Fatalf("unexpected conflict: %v", problem)
	}
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}
