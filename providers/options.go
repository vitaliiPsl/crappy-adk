package providers

import "github.com/vitaliiPsl/crappy-adk/kit"

// ModelOptions configures model provider construction.
type ModelOptions struct {
	APIKey  string
	BaseURL string
	Config  kit.ModelConfig
}

// ModelOption customizes model provider construction.
type ModelOption func(*ModelOptions)

// WithAPIKey authenticates the provider with an API key.
func WithAPIKey(apiKey string) ModelOption {
	return func(options *ModelOptions) {
		options.APIKey = apiKey
	}
}

// WithBaseURL points a model provider at a compatible endpoint.
func WithBaseURL(baseURL string) ModelOption {
	return func(options *ModelOptions) {
		options.BaseURL = baseURL
	}
}

// WithModelConfig overrides the static metadata returned by [kit.Model.Config].
func WithModelConfig(config kit.ModelConfig) ModelOption {
	return func(options *ModelOptions) {
		options.Config = config
	}
}
