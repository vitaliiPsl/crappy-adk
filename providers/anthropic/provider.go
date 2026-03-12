package anthropic

import (
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const providerID = "anthropic"

var _ kit.Provider = (*Provider)(nil)

// Provider implements [kit.Provider] for the Anthropic API.
type Provider struct {
	client *anthropic.Client
}

func New(apiKey string) *Provider {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &Provider{client: &client}
}

// Model returns the model with the given ID, or an error if it is unknown.
func (p *Provider) Model(id string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == id {
			return &model{client: p.client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("anthropic: unknown model %q", id)
}

// Models returns the list of supported models.
func (p *Provider) Models() ([]kit.ModelConfig, error) {
	return knownModels, nil
}
