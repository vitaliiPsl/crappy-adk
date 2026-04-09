package openaicompat

import (
	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const (
	ProviderID     = "openaicompat"
	DefaultBaseURL = "http://localhost:11434/v1"
)

// New returns a model for any OpenAI Chat Completions-compatible server.
// baseURL defaults to Ollama at localhost:11434 if empty.
// cfg.ID must be set; other fields are optional but enable compaction and cost tracking.
func New(baseURL, apiKey string, cfg kit.ModelConfig) kit.Model {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	cfg.Provider = ProviderID

	client := openaisdk.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey(apiKey),
	)

	return &model{client: &client, config: cfg}
}
