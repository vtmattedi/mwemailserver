package store

import (
	"context"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/domain"
)

type EmailAttempt struct {
	ID                 int64
	ProjectID          int64
	Provider           string
	ProviderID         string
	Status             string
	IdempotencyKeyHash string
	RequestHash        string
	CreatedAt          time.Time
}

type ReserveEmailAttemptParams struct {
	ProjectID          int64
	Provider           string
	Sender             string
	Subject            string
	RecipientCount     int
	IdempotencyKeyHash string
	RequestHash        string
}

type ProcessEmailEventParams struct {
	Provider        string
	ProviderEventID string
	ProviderEmailID string
	EventType       string
	OccurredAt      time.Time
	DeliveryStatus  string
}

type Store interface {
	GetProjectByTokenHash(
		ctx context.Context,
		hash string,
	) (*domain.Project, error)

	ReserveEmailAttempt(
		ctx context.Context,
		params ReserveEmailAttemptParams,
	) (attempt EmailAttempt, created bool, err error)

	ResetEmailAttemptForRetry(
		ctx context.Context,
		id int64,
	) error

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

	ProcessEmailEvent(
		ctx context.Context,
		params ProcessEmailEventParams,
	) (duplicate bool, err error)
}
