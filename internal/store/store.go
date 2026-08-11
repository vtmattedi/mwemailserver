package store

import (
	"context"

	"github.com/vtmattedi/mw-email-server/internal/domain"
)

type CreateEmailAttemptParams struct {
	ProjectID      int64
	Sender         string
	Subject        string
	RecipientCount int
}

type RecordEmailEventParams struct {
	EmailAttemptID  *int64
	Provider        string
	ProviderEventID string
	EventType       string
	OccurredAt      string
}

type Store interface {
	GetProjectByTokenHash(
		ctx context.Context,
		hash string,
	) (*domain.Project, error)

	CreateEmailAttempt(
		ctx context.Context,
		params CreateEmailAttemptParams,
	) (int64, error)

	MarkEmailSent(
		ctx context.Context,
		id int64,
		providerID string,
	) error

	MarkEmailFailed(
		ctx context.Context,
		id int64,
		errorMessage string,
	) error

	RecordEmailEvent(
		ctx context.Context,
		params RecordEmailEventParams,
	) error
}
