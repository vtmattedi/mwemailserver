package email

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/store"
)

const providerIdempotencyWindow = 24 * time.Hour

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
	idempotencyKey string,
	message Message,
) (*SendResult, error) {
	keyHash := HashIdempotencyKey(idempotencyKey)

	requestHash, err := HashMessage(message)
	if err != nil {
		return nil, err
	}

	attempt, created, err := s.store.ReserveEmailAttempt(
		ctx,
		store.ReserveEmailAttemptParams{
			ProjectID:          projectID,
			Provider:           "resend",
			Sender:             message.From,
			Subject:            message.Subject,
			RecipientCount:     len(message.To),
			IdempotencyKeyHash: keyHash,
			RequestHash:        requestHash,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("reserve email attempt: %w", err)
	}

	if !created {
		if attempt.RequestHash != requestHash {
			return nil, ErrIdempotencyConflict
		}

		if attempt.Status == "sent" && attempt.ProviderID != "" {
			return &SendResult{
				ProviderID: attempt.ProviderID,
				Replayed:   true,
			}, nil
		}

		age := time.Since(attempt.CreatedAt)
		if age < 0 {
			age = 0
		}

		if age >= providerIdempotencyWindow {
			return nil, ErrIdempotencyWindowExpired
		}

		if err := s.store.ResetEmailAttemptForRetry(ctx, attempt.ID); err != nil {
			return nil, fmt.Errorf("prepare idempotent retry: %w", err)
		}
	}

	providerKey := ProviderIdempotencyKey(projectID, keyHash)

	result, err := s.provider.Send(ctx, message, providerKey)
	if err != nil {
		if updateErr := s.store.MarkEmailFailed(ctx, attempt.ID, err.Error()); updateErr != nil {
			slog.Error(
				"failed to mark email attempt as failed",
				"attempt_id", attempt.ID,
				"error", updateErr,
			)
		}

		return nil, fmt.Errorf("send email: %w", err)
	}

	if err := s.store.MarkEmailSent(ctx, attempt.ID, result.ProviderID); err != nil {
		return nil, fmt.Errorf(
			"provider accepted email %s but database update failed: %w",
			result.ProviderID,
			err,
		)
	}

	result.Replayed = !created
	return result, nil
}
