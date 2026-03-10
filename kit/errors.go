package kit

import (
	"errors"
	"fmt"
)

var (
	ErrRateLimit      = errors.New("rate limit exceeded")
	ErrContextLength  = errors.New("context length exceeded")
	ErrAuthentication = errors.New("authentication failed")
	ErrInvalidRequest = errors.New("invalid request")
	ErrServerError    = errors.New("server error")
)

// LLMError is a structured error returned by model providers.
// Use errors.As to unwrap it, or check [LLMError.Kind] against the sentinel errors above.
type LLMError struct {
	// One of the sentinel errors (e.g. [ErrRateLimit], [ErrServerError]).
	Kind error
	// Human-readable message from the provider.
	Message string
	// HTTP status code, if available.
	StatusCode int
	// Whether the request can be retried.
	Retryable bool
	// Name of the provider that returned the error.
	Provider string
	// The underlying cause, if any.
	Cause error
}

func (e *LLMError) Error() string {
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *LLMError) Unwrap() error {
	return e.Kind
}
