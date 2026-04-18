package openai

import (
	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the OpenAI provider.
const ProviderID = "openai"

type options struct {
	baseURL string
}

// Option customizes the OpenAI provider.
type Option func(*options)

// WithBaseURL points the provider at an OpenAI-compatible endpoint.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// New returns an authenticated model for the given modelID and apiKey.
// Unknown model IDs are allowed so callers can target compatible backends that
// expose models not listed in the built-in catalog.
func New(apiKey, modelID string, opts ...Option) (kit.Model, error) {
	cfg := modelConfig(modelID)

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

	client := openaisdk.NewClient(clientOptions...)

	return &model{client: &client, config: cfg}, nil
}

func modelConfig(modelID string) kit.ModelConfig {
	for _, cfg := range knownModels {
		if cfg.ID == modelID {
			return cfg
		}
	}

	return kit.ModelConfig{
		ID:       modelID,
		Provider: ProviderID,
	}
}

// Models returns the list of supported models.
func Models() []kit.ModelConfig {
	return knownModels
}
