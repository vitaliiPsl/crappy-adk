package kit

import "errors"

var (
	ErrRateLimit      = errors.New("rate limit exceeded")
	ErrContextLength  = errors.New("context length exceeded")
	ErrAuthentication = errors.New("authentication failed")
	ErrInvalidRequest = errors.New("invalid request")
	ErrServerError    = errors.New("server error")
)
