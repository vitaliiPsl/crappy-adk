package openai

import (
	"fmt"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the OpenAI provider.
const ProviderID = "openai"

// New returns an authenticated model for the given modelID and apiKey.
func New(apiKey, modelID string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == modelID {
			client := openaisdk.NewClient(option.WithAPIKey(apiKey))

			return &model{client: &client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("openai: unknown model %q", modelID)
}

// Models returns the list of supported models.
func Models() []kit.ModelConfig {
	return knownModels
}
