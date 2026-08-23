package retry

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Model = (*model)(nil)

// Retryable reports whether err is worth another attempt.
// Unclassified errors are retried; transport failures rarely map to a sentinel.
func Retryable(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return false
	case kit.IsStopped(err):
		return false
	case errors.Is(err, kit.ErrRateLimit), errors.Is(err, kit.ErrServerError):
		return true
	case errors.Is(err, kit.ErrAuthentication),
		errors.Is(err, kit.ErrInvalidRequest),
		errors.Is(err, kit.ErrContextLength),
		errors.Is(err, kit.ErrInvalidOutput):
		return false
	default:
		return true
	}
}

// Option customizes behavior that configuration cannot express.
type Option func(*model)

// WithRetryable replaces [Retryable] as the error classifier.
func WithRetryable(retryable func(error) bool) Option {
	return func(m *model) {
		if retryable != nil {
			m.retryable = retryable
		}
	}
}

// WithOnRetry observes each retry.
func WithOnRetry(observe func(Attempt)) Option {
	return func(m *model) {
		m.onRetry = observe
	}
}

// Wrap returns a model that retries failed calls, and wrapped unchanged when
// config disables retrying. Streams are retried only until the first event
// reaches the consumer, after which a replay would duplicate content.
func Wrap(wrapped kit.Model, config Config, options ...Option) kit.Model {
	if !config.enabled() {
		return wrapped
	}

	retrier := &model{
		model:     wrapped,
		config:    config.withDefaults(),
		retryable: Retryable,
	}

	for _, option := range options {
		option(retrier)
	}

	return retrier
}

type model struct {
	model     kit.Model
	config    Config
	retryable func(error) bool
	onRetry   func(Attempt)
}

func (m *model) Config() kit.ModelConfig {
	return m.model.Config()
}

func (m *model) Generate(ctx context.Context, request kit.ModelRequest) (kit.ModelResponse, error) {
	var response kit.ModelResponse

	err := m.do(ctx, func() error {
		var err error

		response, err = m.model.Generate(ctx, request)

		return err
	})

	return response, err
}

func (m *model) Stream(
	ctx context.Context,
	request kit.ModelRequest,
) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.ModelEvent]) (kit.ModelResponse, error) {
		var (
			response  kit.ModelResponse
			forwarded bool
		)

		err := m.do(ctx, func() error {
			stream := m.model.Stream(ctx, request)

			for event := range stream.Iter() {
				forwarded = true

				if err := emit.Emit(event); err != nil {
					return err
				}
			}

			var err error

			response, err = stream.Result()

			return err
		}, func(error) bool { return !forwarded })

		return response, err
	})
}

func (m *model) do(ctx context.Context, attempt func() error, extra ...func(error) bool) error {
	var (
		started = time.Now()
		delay   = m.config.BaseDelay
	)

	for number := 1; ; number++ {
		err := attempt()
		if err == nil {
			return nil
		}

		if number >= m.config.MaxAttempts || !m.canRetry(err, extra) {
			return err
		}

		elapsed := time.Since(started)
		if elapsed+delay > m.config.Budget {
			return err
		}

		if m.onRetry != nil {
			m.onRetry(Attempt{
				Number:  number,
				Err:     err,
				Delay:   delay,
				Elapsed: elapsed,
			})
		}

		if err := sleep(ctx, delay); err != nil {
			return errors.Join(err, ctx.Err())
		}

		delay = min(delay*2, m.config.MaxDelay)
	}
}

func (m *model) canRetry(err error, extra []func(error) bool) bool {
	for _, allows := range extra {
		if !allows(err) {
			return false
		}
	}

	return m.retryable(err)
}

func sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(jitter(delay))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func jitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}

	return delay - time.Duration(rand.Int64N(int64(delay)/4+1))
}
