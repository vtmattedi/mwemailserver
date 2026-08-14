package federation

import "errors"

var (
	ErrInvalidToken     = errors.New("invalid federated token")
	ErrAudienceMismatch = errors.New("federated token audience mismatch")
	ErrScopeMissing     = errors.New("required federated scope missing")
)
