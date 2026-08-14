package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestListProjectEmailCounts(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "counts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	_, err = store.db.ExecContext(ctx, `
		INSERT INTO projects (name, token_hash, enabled) VALUES
			('Empty', 'empty-token', 1),
			('Used', 'used-token', 0)
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.db.ExecContext(ctx, `
		INSERT INTO email_attempts (
			project_id, sender, subject, recipient_count, status, delivery_status
		) VALUES
			(2, 'sender@example.com', 'one', 1, 'sent', 'delivered'),
			(2, 'sender@example.com', 'two', 1, 'failed', 'bounced'),
			(2, 'sender@example.com', 'three', 1, 'pending', NULL)
	`)
	if err != nil {
		t.Fatal(err)
	}

	projects, err := store.ListProjectEmailCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(projects))
	}

	empty := projects[0]
	if empty.Name != "Empty" || empty.EmailCount != 0 || !empty.Enabled {
		t.Fatalf("unexpected empty project: %+v", empty)
	}

	used := projects[1]
	if used.Name != "Used" || used.Enabled || used.EmailCount != 3 {
		t.Fatalf("unexpected used project: %+v", used)
	}
	if used.Send.Pending != 1 || used.Send.Sent != 1 || used.Send.Failed != 1 {
		t.Fatalf("unexpected send counts: %+v", used.Send)
	}
	if used.Delivery.Delivered != 1 || used.Delivery.Bounced != 1 {
		t.Fatalf("unexpected delivery counts: %+v", used.Delivery)
	}
}
