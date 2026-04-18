package anthropic

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the Anthropic provider.
const ProviderID = "anthropic"

type options struct {
	baseURL string
}

// Option customizes the Anthropic provider.
type Option func(*options)

// WithBaseURL points the provider at an Anthropic Messages-compatible endpoint.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// New returns an authenticated model for the given modelID and apiKey.
func New(apiKey, modelID string, opts ...Option) (kit.Model, error) {
	return NewWithConfig(apiKey, kit.ModelConfig{
		ID:       modelID,
		Provider: ProviderID,
	}, opts...)
}

// NewWithConfig returns an authenticated model using the provided static
// model metadata.
func NewWithConfig(apiKey string, cfg kit.ModelConfig, opts ...Option) (kit.Model, error) {
	if cfg.Provider == "" {
		cfg.Provider = ProviderID
	}

	options := options{}
	for _, opt := range opts {
		opt(&options)
	}

	clientOptions := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if options.baseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(options.baseURL))
	}

	client := anthropic.NewClient(clientOptions...)

	return &model{client: &client, config: cfg}, nil
}
