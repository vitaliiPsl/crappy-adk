package google

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the Google provider.
const ProviderID = "google"

var _ kit.Provider = (*Provider)(nil)

// Provider implements [kit.Provider] for the Google Gemini API.
type Provider struct{}

// New creates a Google provider.
func New() *Provider {
	return &Provider{}
}

// Model returns an authenticated model for the given ID and API key.
func (p *Provider) Model(ctx context.Context, id string, apiKey string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == id {
			client, err := genai.NewClient(ctx, &genai.ClientConfig{
				APIKey:  apiKey,
				Backend: genai.BackendGeminiAPI,
			})
			if err != nil {
				return nil, fmt.Errorf("google: create client: %w", err)
			}

			return &model{client: client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("google: unknown model %q", id)
}

// Models returns the list of supported models.
func (p *Provider) Models() []kit.ModelConfig {
	return knownModels
}
