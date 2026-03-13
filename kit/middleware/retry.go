package middleware

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const (
	defaultMaxAttempts = 3
	defaultBaseDelay   = 500 * time.Millisecond
	defaultMaxDelay    = 30 * time.Second
)

// RetryOption configures the retry middleware.
type RetryOption func(*retryModel)

// WithMaxAttempts sets the maximum number of attempts, including the first.
func WithMaxAttempts(n int) RetryOption {
	return func(r *retryModel) {
		r.maxAttempts = n
	}
}

// WithBaseDelay sets the base delay for exponential backoff.
func WithBaseDelay(d time.Duration) RetryOption {
	return func(r *retryModel) {
		r.baseDelay = d
	}
}

// WithMaxDelay sets the upper bound on the backoff delay.
func WithMaxDelay(d time.Duration) RetryOption {
	return func(r *retryModel) {
		r.maxDelay = d
	}
}

type retryModel struct {
	BaseModel
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

// Retry wraps model with exponential-backoff retry logic. Only errors where
// [kit.LLMError.Retryable] is true are retried.
//
// For streaming calls, only the initial connection is retried. Mid-stream
// errors are passed through to the caller as-is.
func Retry(model kit.Model, opts ...RetryOption) kit.Model {
	r := &retryModel{
		BaseModel:   BaseModel{Next: model},
		maxAttempts: defaultMaxAttempts,
		baseDelay:   defaultBaseDelay,
		maxDelay:    defaultMaxDelay,
	}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *retryModel) Generate(ctx context.Context, req kit.ModelRequest) (kit.ModelResponse, error) {
	var lastErr error
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if attempt > 0 {
			if err := r.wait(ctx, attempt); err != nil {
				return kit.ModelResponse{}, err
			}
		}

		resp, err := r.Next.Generate(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	return kit.ModelResponse{}, lastErr
}

func (r *retryModel) GenerateStream(ctx context.Context, req kit.ModelRequest) (*kit.ModelStream, error) {
	var lastErr error
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if attempt > 0 {
			if err := r.wait(ctx, attempt); err != nil {
				return nil, err
			}
		}

		s, err := r.Next.GenerateStream(ctx, req)
		if err == nil || !isRetryable(err) {
			return s, err
		}

		lastErr = err
	}

	return nil, lastErr
}

// wait sleeps for the backoff duration for the given attempt, respecting context cancellation.
func (r *retryModel) wait(ctx context.Context, attempt int) error {
	delay := r.backoff(attempt)
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// backoff returns the delay for the given attempt using exponential backoff
func (r *retryModel) backoff(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt-1))
	cp := min(time.Duration(exp)*r.baseDelay, r.maxDelay)

	return time.Duration(rand.Int64N(int64(cp)))
}

func isRetryable(err error) bool {
	var llmErr *kit.LLMError
	if errors.As(err, &llmErr) {
		return llmErr.Retryable
	}

	return false
}
