package custom

import (
	"context"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const (
	ProviderID     = "custom"
	DefaultBaseURL = "http://localhost:11434/v1"
)

var _ kit.Provider = (*Provider)(nil)

// Provider implements [kit.Provider] for custom OpenAI-compatible inference servers.
type Provider struct {
	baseURL string
}

// New creates a provider targeting the given base URL. Defaults to Ollama'.
func New(baseURL string) *Provider {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &Provider{baseURL: baseURL}
}

// Model returns a model for the given ID.
func (p *Provider) Model(_ context.Context, id string, apiKey string) (kit.Model, error) {
	client := openaisdk.NewClient(
		option.WithBaseURL(p.baseURL),
		option.WithAPIKey(apiKey),
	)

	cfg := kit.ModelConfig{
		ID:       id,
		Provider: ProviderID,
		Capabilities: kit.ModelCapabilities{
			Text:      true,
			Tools:     true,
			Streaming: true,
		},
	}

	return &model{client: &client, config: cfg}, nil
}

func (p *Provider) Models() []kit.ModelConfig {
	return nil
}
