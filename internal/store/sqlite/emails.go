package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/store"
)

func (s *Store) ReserveEmailAttempt(
	ctx context.Context,
	params store.ReserveEmailAttemptParams,
) (store.EmailAttempt, bool, error) {
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO email_attempts (
			project_id,
			provider,
			sender,
			subject,
			recipient_count,
			status,
			idempotency_key_hash,
			request_hash
		) VALUES (?, ?, ?, ?, ?, 'pending', ?, ?)`,
		params.ProjectID,
		params.Provider,
		params.Sender,
		params.Subject,
		params.RecipientCount,
		params.IdempotencyKeyHash,
		params.RequestHash,
	)
	if err == nil {
		id, idErr := result.LastInsertId()
		if idErr != nil {
			return store.EmailAttempt{}, false, fmt.Errorf("get email attempt id: %w", idErr)
		}

		attempt, getErr := s.getEmailAttemptByID(ctx, id)
		if getErr != nil {
			return store.EmailAttempt{}, false, getErr
		}

		return attempt, true, nil
	}

	// The expected failure here is the unique project/idempotency-key constraint
	// racing with another copy of the same request. Query the existing row.
	attempt, getErr := s.getEmailAttemptByIdempotency(
		ctx,
		params.ProjectID,
		params.IdempotencyKeyHash,
	)
	if getErr == nil {
		return attempt, false, nil
	}

	if getErr != sql.ErrNoRows {
		return store.EmailAttempt{}, false, fmt.Errorf(
			"lookup existing idempotent email attempt: %w",
			getErr,
		)
	}

	return store.EmailAttempt{}, false, fmt.Errorf("insert email attempt: %w", err)
}

func (s *Store) ResetEmailAttemptForRetry(
	ctx context.Context,
	id int64,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE email_attempts
		 SET status = 'pending',
		     error_message = NULL,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("reset email attempt: %w", err)
	}

	return nil
}

func (s *Store) MarkEmailSent(
	ctx context.Context,
	id int64,
	providerID string,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE email_attempts
		 SET status = 'sent',
		     provider_id = ?,
		     error_message = NULL,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		providerID,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark email sent: %w", err)
	}

	return nil
}

func (s *Store) MarkEmailFailed(
	ctx context.Context,
	id int64,
	errorMessage string,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE email_attempts
		 SET status = 'failed',
		     error_message = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		errorMessage,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark email failed: %w", err)
	}

	return nil
}

func (s *Store) getEmailAttemptByIdempotency(
	ctx context.Context,
	projectID int64,
	keyHash string,
) (store.EmailAttempt, error) {
	return scanEmailAttempt(
		s.db.QueryRowContext(
			ctx,
			`SELECT
				id,
				project_id,
				provider,
				COALESCE(provider_id, ''),
				status,
				COALESCE(idempotency_key_hash, ''),
				COALESCE(request_hash, ''),
				created_at
			 FROM email_attempts
			 WHERE project_id = ?
			   AND idempotency_key_hash = ?
			 LIMIT 1`,
			projectID,
			keyHash,
		),
	)
}

func (s *Store) getEmailAttemptByID(
	ctx context.Context,
	id int64,
) (store.EmailAttempt, error) {
	return scanEmailAttempt(
		s.db.QueryRowContext(
			ctx,
			`SELECT
				id,
				project_id,
				provider,
				COALESCE(provider_id, ''),
				status,
				COALESCE(idempotency_key_hash, ''),
				COALESCE(request_hash, ''),
				created_at
			 FROM email_attempts
			 WHERE id = ?
			 LIMIT 1`,
			id,
		),
	)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEmailAttempt(row rowScanner) (store.EmailAttempt, error) {
	var attempt store.EmailAttempt
	var createdAt string

	if err := row.Scan(
		&attempt.ID,
		&attempt.ProjectID,
		&attempt.Provider,
		&attempt.ProviderID,
		&attempt.Status,
		&attempt.IdempotencyKeyHash,
		&attempt.RequestHash,
		&createdAt,
	); err != nil {
		return store.EmailAttempt{}, err
	}

	parsed, err := parseSQLiteTime(createdAt)
	if err != nil {
		return store.EmailAttempt{}, fmt.Errorf("parse email attempt created_at: %w", err)
	}

	attempt.CreatedAt = parsed
	return attempt, nil
}

func parseSQLiteTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
	}

	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastErr = err
	}

	return time.Time{}, lastErr
}
