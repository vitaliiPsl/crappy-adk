package providers

import (
	"maps"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ModelOptions configures model provider construction.
type ModelOptions struct {
	Config      kit.ModelConfig
	BaseURL     string
	APIKey      string
	BearerToken string
	Headers     map[string]string
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

// WithAPIKey authenticates the provider with an API key.
func WithAPIKey(apiKey string) ModelOption {
	return func(options *ModelOptions) {
		options.APIKey = apiKey
	}
}

// WithBearerToken authenticates the provider with a bearer token. It takes
// precedence over an API key when the provider supports both.
func WithBearerToken(token string) ModelOption {
	return func(options *ModelOptions) {
		options.BearerToken = token
	}
}

// WithHeaders adds headers to every model request.
func WithHeaders(headers map[string]string) ModelOption {
	return func(options *ModelOptions) {
		options.Headers = make(map[string]string, len(headers))
		maps.Copy(options.Headers, headers)
	}
}
