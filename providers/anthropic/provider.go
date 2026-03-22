package anthropic

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the Anthropic provider.
const ProviderID = "anthropic"

var _ kit.Provider = (*Provider)(nil)

// Provider implements [kit.Provider] for the Anthropic API.
type Provider struct{}

// New creates an Anthropic provider.
func New() *Provider {
	return &Provider{}
}

// Model returns an authenticated model for the given ID and API key.
func (p *Provider) Model(ctx context.Context, id string, apiKey string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == id {
			client := anthropic.NewClient(option.WithAPIKey(apiKey))
			return &model{client: &client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("anthropic: unknown model %q", id)
}

// Models returns the list of supported models.
func (p *Provider) Models() []kit.ModelConfig {
	return knownModels
}
