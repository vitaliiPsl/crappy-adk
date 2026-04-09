package google

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// ProviderID uniquely identifies the Google provider.
const ProviderID = "google"

// New returns an authenticated model for the given modelID and apiKey.
func New(apiKey, modelID string) (kit.Model, error) {
	for _, cfg := range knownModels {
		if cfg.ID == modelID {
			client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
				APIKey:  apiKey,
				Backend: genai.BackendGeminiAPI,
			})
			if err != nil {
				return nil, fmt.Errorf("google: create client: %w", err)
			}

			return &model{client: client, config: cfg}, nil
		}
	}

	return nil, fmt.Errorf("google: unknown model %q", modelID)
}

// Models returns the list of supported models.
func Models() []kit.ModelConfig {
	return knownModels
}
