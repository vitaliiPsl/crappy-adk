package google

import (
	"context"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the Google provider.
const ProviderID = "google"

type options struct {
	baseURL string
	backend genai.Backend
}

// Option customizes the Google provider.
type Option func(*options)

// WithBaseURL points the provider at a Gemini-compatible endpoint.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// WithBackend overrides the Google GenAI SDK backend selection.
func WithBackend(backend genai.Backend) Option {
	return func(o *options) {
		o.backend = backend
	}
}

// New returns an authenticated model for the given modelID and apiKey.
// Unknown model IDs are allowed so callers can target compatible gateways that
// expose models not listed in the built-in catalog.
func New(apiKey, modelID string, opts ...Option) (kit.Model, error) {
	cfg := modelConfig(modelID)

	options := options{
		backend: genai.BackendGeminiAPI,
	}
	for _, opt := range opts {
		opt(&options)
	}

	clientConfig := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: options.backend,
	}
	if options.baseURL != "" {
		clientConfig.HTTPOptions = genai.HTTPOptions{BaseURL: options.baseURL}
	}

	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		return nil, err
	}

	return &model{client: client, config: cfg}, nil
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
