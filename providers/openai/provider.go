package openai

import (
	"context"
	"fmt"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the OpenAI provider.
const ProviderID = "openai"

var _ kit.Provider = (*Provider)(nil)

// Provider implements [kit.Provider] for the OpenAI Responses API.
type Provider struct{}

// New creates an OpenAI provider.
func New() *Provider {
	return &Provider{}
}

// Model returns an authenticated model for the given ID and API key.
func (p *Provider) Model(_ context.Context, id string, apiKey string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == id {
			client := openaisdk.NewClient(option.WithAPIKey(apiKey))

			return &model{client: &client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("openai: unknown model %q", id)
}

// Models returns the list of supported models.
func (p *Provider) Models() []kit.ModelConfig {
	return knownModels
}
