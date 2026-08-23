package retry

import "time"

const (
	DefaultBudget      = time.Minute
	DefaultMaxAttempts = 6
	DefaultBaseDelay   = time.Second
	DefaultMaxDelay    = 30 * time.Second
)

// Attempt describes a failed call that is about to be retried.
type Attempt struct {
	// Number is the 1-based index of the attempt that failed.
	Number int
	// Err is the error that attempt returned.
	Err error
	// Delay is how long the next attempt waits.
	Delay time.Duration
	// Elapsed is the time spent since the first attempt started.
	Elapsed time.Duration
}

// Config controls how a wrapped model retries failed calls.
type Config struct {
	// Budget is the wall-clock ceiling on retrying a single call.
	// A non-positive budget disables retrying.
	Budget time.Duration `json:"budget,omitempty" yaml:"budget,omitempty"`
	// MaxAttempts caps total attempts. A value below two disables retrying.
	MaxAttempts int `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	// BaseDelay is the delay before the second attempt.
	BaseDelay time.Duration `json:"base_delay,omitempty" yaml:"base_delay,omitempty"`
	// MaxDelay caps the exponential backoff.
	MaxDelay time.Duration `json:"max_delay,omitempty" yaml:"max_delay,omitempty"`
}

// DefaultConfig returns the configuration applied when none is provided.
func DefaultConfig() Config {
	return Config{
		Budget:      DefaultBudget,
		MaxAttempts: DefaultMaxAttempts,
		BaseDelay:   DefaultBaseDelay,
		MaxDelay:    DefaultMaxDelay,
	}
}

func (c Config) enabled() bool {
	return c.Budget > 0 && c.MaxAttempts > 1
}

func (c Config) withDefaults() Config {
	if c.BaseDelay <= 0 {
		c.BaseDelay = DefaultBaseDelay
	}

	if c.MaxDelay < c.BaseDelay {
		c.MaxDelay = c.BaseDelay
	}

	return c
}
