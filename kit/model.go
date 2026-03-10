package kit

import (
	"context"
)

type Provider interface {
	Model(id string) (Model, error)
	Models() ([]ModelConfig, error)
}

type Model interface {
	Config() ModelConfig
	Generate(ctx context.Context, req ModelRequest) (ModelResponse, error)
}

type ModelConfig struct {
	ID              string
	Provider        string
	ContextWindow   int
	MaxOutputTokens int
	InputPricePerM  float64
	OutputPricePerM float64
}

type ModelRequest struct {
	Instructions string
	Messages     []Message
	Tools        []ToolDefinition
}

type ModelResponse struct {
	Content   string
	ToolCalls []ToolCall
}
