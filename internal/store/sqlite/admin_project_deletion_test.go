package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/admin"
)

func TestDeleteAdminProjectIsAtomicAndAudited(t *testing.T) {
	dbStore, err := Open(filepath.Join(t.TempDir(), "delete.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbStore.Close()

	ctx := context.Background()
	project, err := dbStore.CreateAdminProject(ctx, "Delete me", "token-to-revoke", nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := dbStore.CreateAdminProject(ctx, "Keep me", "other-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	subject := "usr_delete_actor"
	occurredAt := time.Date(2026, 8, 14, 12, 30, 0, 0, time.UTC)

	deleted, err := dbStore.DeleteAdminProject(ctx, admin.DeleteProjectParams{
		ID:              project.ID,
		ActorType:       "federated",
		IdentitySubject: &subject,
		RequestID:       "req-delete-1",
		OccurredAt:      occurredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted.ID != project.ID || deleted.Name != project.Name {
		t.Fatalf("unexpected deleted project: %+v", deleted)
	}

	var projectCount int
	if err := dbStore.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id = ?`, project.ID).Scan(&projectCount); err != nil {
		t.Fatal(err)
	}
	if projectCount != 0 {
		t.Fatal("deleted project and credential row still exist")
	}
	if err := dbStore.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id = ?`, other.ID).Scan(&projectCount); err != nil {
		t.Fatal(err)
	}
	if projectCount != 1 {
		t.Fatal("deletion affected another project")
	}

	var eventType, actorType, actorSubject, projectName, requestID, timestamp string
	var auditProjectID int64
	if err := dbStore.db.QueryRowContext(ctx, `
		SELECT event_type, actor_type, actor_identity_sub,
		       project_id, project_name, request_id, occurred_at
		FROM admin_audit_events
		WHERE project_id = ?`, project.ID,
	).Scan(&eventType, &actorType, &actorSubject, &auditProjectID, &projectName, &requestID, &timestamp); err != nil {
		t.Fatal(err)
	}
	if eventType != "project.deleted" || actorType != "federated" || actorSubject != subject ||
		auditProjectID != project.ID || projectName != project.Name || requestID != "req-delete-1" || timestamp != occurredAt.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected audit event: %q %q %q %d %q %q %q", eventType, actorType, actorSubject, auditProjectID, projectName, requestID, timestamp)
	}

	if _, err := dbStore.db.ExecContext(ctx, `UPDATE admin_audit_events SET request_id = 'changed' WHERE project_id = ?`, project.ID); err == nil {
		t.Fatal("immutable audit event accepted an update")
	}
	if _, err := dbStore.db.ExecContext(ctx, `DELETE FROM admin_audit_events WHERE project_id = ?`, project.ID); err == nil {
		t.Fatal("immutable audit event accepted deletion")
	}

	_, err = dbStore.DeleteAdminProject(ctx, admin.DeleteProjectParams{
		ID: project.ID, ActorType: "federated", IdentitySubject: &subject, OccurredAt: occurredAt,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("repeated deletion returned %v, want sql.ErrNoRows", err)
	}
}

func TestDeleteAdminProjectRetentionAndAuditFailureRollback(t *testing.T) {
	dbStore, err := Open(filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbStore.Close()

	ctx := context.Background()
	subject := "usr_delete_actor"
	retained, err := dbStore.CreateAdminProject(ctx, "Retained", "retained-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbStore.db.ExecContext(ctx, `
		INSERT INTO email_attempts (project_id, sender, subject, recipient_count)
		VALUES (?, 'sender@example.com', 'retained email', 1)`, retained.ID,
	); err != nil {
		t.Fatal(err)
	}

	_, err = dbStore.DeleteAdminProject(ctx, admin.DeleteProjectParams{
		ID: retained.ID, ActorType: "federated", IdentitySubject: &subject, OccurredAt: time.Now().UTC(),
	})
	if !errors.Is(err, admin.ErrProjectHasDependencies) {
		t.Fatalf("got %v, want ErrProjectHasDependencies", err)
	}

	rollback, err := dbStore.CreateAdminProject(ctx, "Rollback", "rollback-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = dbStore.DeleteAdminProject(ctx, admin.DeleteProjectParams{
		ID: rollback.ID, ActorType: "invalid", IdentitySubject: &subject, OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected invalid audit actor to fail")
	}

	for _, id := range []int64{retained.ID, rollback.ID} {
		var count int
		if err := dbStore.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id = ?`, id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("project %d was not preserved after failed deletion", id)
		}
	}
}
