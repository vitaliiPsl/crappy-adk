package openai

import (
	"fmt"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const providerID = "openai"

// Provider implements [kit.Provider] for the OpenAI Responses API.
type Provider struct {
	client *openaisdk.Client
}

var _ kit.Provider = (*Provider)(nil)

// New creates an OpenAI provider authenticated with the given API key.
func New(apiKey string) *Provider {
	client := openaisdk.NewClient(option.WithAPIKey(apiKey))
	return &Provider{client: &client}
}

// Model returns the model with the given ID, or an error if it is unknown.
func (p *Provider) Model(id string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == id {
			return &model{client: p.client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("openai: unknown model %q", id)
}

// Models returns the list of supported models.
func (p *Provider) Models() ([]kit.ModelConfig, error) {
	return knownModels, nil
}
