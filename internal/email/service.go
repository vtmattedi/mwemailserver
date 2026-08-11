package email

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/vtmattedi/mw-email-server/internal/store"
)

type Service struct {
	provider Provider
	store    store.Store
}

func NewService(provider Provider, store store.Store) *Service {
	return &Service{
		provider: provider,
		store:    store,
	}
}

func (s *Service) Send(
	ctx context.Context,
	projectID int64,
	message Message,
) (*SendResult, error) {
	attemptID, err := s.store.CreateEmailAttempt(ctx, store.CreateEmailAttemptParams{
		ProjectID:      projectID,
		Sender:         message.From,
		Subject:        message.Subject,
		RecipientCount: len(message.To),
	})
	if err != nil {
		return nil, fmt.Errorf("create email attempt: %w", err)
	}

	result, err := s.provider.Send(ctx, message)
	if err != nil {
		if updateErr := s.store.MarkEmailFailed(ctx, attemptID, err.Error()); updateErr != nil {
			slog.Error(
				"failed to mark email attempt as failed",
				"attempt_id", attemptID,
				"error", updateErr,
			)
		}

		return nil, fmt.Errorf("send email: %w", err)
	}

	if err := s.store.MarkEmailSent(ctx, attemptID, result.ProviderID); err != nil {
		slog.Error(
			"email was sent but attempt status could not be updated",
			"attempt_id", attemptID,
			"provider_id", result.ProviderID,
			"error", err,
		)
	}

	return result, nil
}
