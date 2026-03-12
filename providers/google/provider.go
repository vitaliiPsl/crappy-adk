package google

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const providerID = "google"

var _ kit.Provider = (*Provider)(nil)

// Provider implements [kit.Provider] for the Google Gemini API.
type Provider struct {
	client *genai.Client
}

// New creates a Google provider authenticated with the given API key.
func New(ctx context.Context, apiKey string) (*Provider, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("google: create client: %w", err)
	}

	return &Provider{client: client}, nil
}

// Model returns the model with the given ID, or an error if it is unknown.
func (p *Provider) Model(id string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == id {
			return &model{client: p.client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("google: unknown model %q", id)
}

// Models returns the list of supported models.
func (p *Provider) Models() ([]kit.ModelConfig, error) {
	return knownModels, nil
}
