package sqlite

import (
	"context"
	"fmt"

	"github.com/vtmattedi/mw-email-server/internal/store"
)

func (s *Store) RecordEmailEvent(
	ctx context.Context,
	params store.RecordEmailEventParams,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO email_events (
			email_attempt_id,
			provider,
			provider_event_id,
			event_type,
			occurred_at
		) VALUES (?, ?, ?, ?, ?)`,
		params.EmailAttemptID,
		params.Provider,
		nullableString(params.ProviderEventID),
		params.EventType,
		nullableString(params.OccurredAt),
	)
	if err != nil {
		return fmt.Errorf("insert email event: %w", err)
	}

	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}
