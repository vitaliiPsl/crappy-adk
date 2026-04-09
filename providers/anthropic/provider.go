package anthropic

import (
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the Anthropic provider.
const ProviderID = "anthropic"

// New returns an authenticated model for the given modelID and apiKey.
func New(apiKey, modelID string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == modelID {
			client := anthropic.NewClient(option.WithAPIKey(apiKey))

			return &model{client: &client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("anthropic: unknown model %q", modelID)
}

// Models returns the list of supported models.
func Models() []kit.ModelConfig {
	return knownModels
}
