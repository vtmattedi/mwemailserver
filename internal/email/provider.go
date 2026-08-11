package email

import "context"

type Provider interface {
	Send(
		ctx context.Context,
		message Message,
		idempotencyKey string,
	) (*SendResult, error)
}
