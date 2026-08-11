package sqlite

import (
	"context"
	"fmt"

	"github.com/vtmattedi/mw-email-server/internal/store"
)

func (s *Store) CreateEmailAttempt(
	ctx context.Context,
	params store.CreateEmailAttemptParams,
) (int64, error) {
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO email_attempts (
			project_id,
			sender,
			subject,
			recipient_count,
			status
		) VALUES (?, ?, ?, ?, 'pending')`,
		params.ProjectID,
		params.Sender,
		params.Subject,
		params.RecipientCount,
	)
	if err != nil {
		return 0, fmt.Errorf("insert email attempt: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get email attempt id: %w", err)
	}

	return id, nil
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
