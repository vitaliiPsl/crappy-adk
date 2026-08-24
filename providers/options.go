package providers

import (
	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ModelOptions configures model provider construction.
type ModelOptions struct {
	Config      kit.ModelConfig
	BaseURL     string
	APIKey      string
	Credentials CredentialsSource
}

// ModelOption customizes model provider construction.
type ModelOption func(*ModelOptions)

// WithModelConfig overrides the static metadata returned by [kit.Model.Config].
func WithModelConfig(config kit.ModelConfig) ModelOption {
	return func(options *ModelOptions) {
		options.Config = config
	}
}

// WithBaseURL points a model provider at a compatible endpoint.
func WithBaseURL(baseURL string) ModelOption {
	return func(options *ModelOptions) {
		options.BaseURL = baseURL
	}
}

// WithAPIKey authorizes every model request with an API key, in whichever
// header the provider expects it.
func WithAPIKey(apiKey string) ModelOption {
	return func(options *ModelOptions) {
		options.APIKey = apiKey
	}
}

// WithCredentials authorizes every model request with headers from source,
// which takes precedence over an API key. A provider left without either
// authenticates the way its own SDK does, usually from the environment.
func WithCredentials(source CredentialsSource) ModelOption {
	return func(options *ModelOptions) {
		options.Credentials = source
	}
}
