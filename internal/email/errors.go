package email

import "errors"

var (
	ErrIdempotencyConflict      = errors.New("idempotency key was already used with a different request")
	ErrIdempotencyWindowExpired = errors.New("previous uncertain send is older than the provider idempotency window")
)
