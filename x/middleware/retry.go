package middleware

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
	xstream "github.com/vitaliiPsl/crappy-adk/x/stream"
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

// NewRetry returns a [kit.ModelMiddleware] that applies retry logic.
func NewRetry(opts ...RetryOption) kit.ModelMiddleware {
	return func(model kit.Model) kit.Model {
		return Retry(model, opts...)
	}
}

// Retry wraps model with exponential-backoff retry logic.
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

// Generate retries the call up to maxAttempts on retryable errors.
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

// GenerateStream retries on retryable errors before the first chunk. Mid-stream errors pass through.
// Immediate errors from the underlying GenerateStream are returned directly, preserving the error contract.
func (r *retryModel) GenerateStream(ctx context.Context, req kit.ModelRequest) (*xstream.Stream[kit.ModelEvent, kit.ModelResponse], error) {
	attempt := 0

	stream, err := r.acquireStream(ctx, req, &attempt)
	if err != nil {
		return nil, err
	}

	return xstream.New(func(yield func(kit.ModelEvent, error) bool) kit.ModelResponse {
		var lastErr error

		for {
			started := false
			retryable := false

			for event, err := range stream.Iter() {
				if err != nil {
					if !started && isRetryable(err) {
						lastErr = err
						attempt++
						retryable = true

						break
					}

					yield(kit.ModelEvent{}, err)

					return kit.ModelResponse{}
				}

				started = true

				if !yield(event, nil) {
					resp, _ := stream.Result()

					return resp
				}
			}

			if !retryable {
				resp, _ := stream.Result()

				return resp
			}

			s, err := r.acquireStream(ctx, req, &attempt)
			if err != nil {
				yield(kit.ModelEvent{}, err)

				return kit.ModelResponse{}
			}

			if s == nil {
				yield(kit.ModelEvent{}, lastErr)

				return kit.ModelResponse{}
			}

			stream = s
		}
	}), nil
}

// acquireStream attempts to get a stream from the underlying model, retrying
// on retryable errors within the shared attempt budget.
func (r *retryModel) acquireStream(ctx context.Context, req kit.ModelRequest, attempt *int) (*xstream.Stream[kit.ModelEvent, kit.ModelResponse], error) {
	var lastErr error

	for *attempt < r.maxAttempts {
		if *attempt > 0 {
			if err := r.wait(ctx, *attempt); err != nil {
				return nil, err
			}
		}

		s, err := r.Next.GenerateStream(ctx, req)
		if err == nil {
			return s, nil
		}

		lastErr = err
		*attempt++

		if !isRetryable(err) {
			return nil, err
		}
	}

	return nil, lastErr
}

// wait sleeps for the backoff duration for the given attempt, respecting context cancellation.
func (r *retryModel) wait(ctx context.Context, attempt int) error {
	delay := r.backoff(attempt)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
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
