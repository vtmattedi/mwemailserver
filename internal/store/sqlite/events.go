package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vtmattedi/mw-email-server/internal/store"
)

func (s *Store) ProcessEmailEvent(
	ctx context.Context,
	params store.ProcessEmailEventParams,
) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin email event transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var attemptID sql.NullInt64

	err = tx.QueryRowContext(
		ctx,
		`SELECT id
		 FROM email_attempts
		 WHERE provider = ?
		   AND provider_id = ?
		 LIMIT 1`,
		params.Provider,
		params.ProviderEmailID,
	).Scan(&attemptID)

	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("find email attempt for event: %w", err)
	}

	result, err := tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO email_events (
			email_attempt_id,
			provider,
			provider_event_id,
			provider_email_id,
			event_type,
			occurred_at,
			received_at
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		nullableInt64(attemptID),
		params.Provider,
		params.ProviderEventID,
		params.ProviderEmailID,
		params.EventType,
		params.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	)
	if err != nil {
		return false, fmt.Errorf("insert email event: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read email event insert result: %w", err)
	}

	if rows == 0 {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit duplicate email event: %w", err)
		}
		return true, nil
	}

	if attemptID.Valid && params.DeliveryStatus != "" {
		occurredAt := params.OccurredAt.UTC().Format(
			"2006-01-02T15:04:05.999999999Z07:00",
		)

		if _, err := tx.ExecContext(
			ctx,
			`UPDATE email_attempts
			 SET delivery_status = ?,
			     delivery_status_at = ?,
			     updated_at = CURRENT_TIMESTAMP
			 WHERE id = ?
			   AND (
			       delivery_status_at IS NULL
			       OR delivery_status_at < ?
			   )`,
			params.DeliveryStatus,
			occurredAt,
			attemptID.Int64,
			occurredAt,
		); err != nil {
			return false, fmt.Errorf("update delivery status: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit email event: %w", err)
	}

	return false, nil
}

func nullableInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}

	return value.Int64
}
