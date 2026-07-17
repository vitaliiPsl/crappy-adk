package anthropic

import (
	"context"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers"
)

var _ kit.Model = (*Model)(nil)

const (
	defaultMaxTokens = 16000
)

var thinkingBudgets = map[kit.ThinkingLevel]int64{
	kit.ThinkingLevelLow:    4000,
	kit.ThinkingLevelMedium: 8000,
	kit.ThinkingLevelHigh:   16000,
}

// Model implements the [kit.Model] interface using Anthropic's messages API.
type Model struct {
	id     string
	config kit.ModelConfig
	client *anthropicsdk.Client
}

// New returns a model for the given ID and options.
func New(id string, opts ...providers.ModelOption) (*Model, error) {
	options := providers.ModelOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	clientOptions := []option.RequestOption{
		option.WithAPIKey(options.APIKey),
	}

	if options.BaseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(options.BaseURL))
	}

	client := anthropicsdk.NewClient(clientOptions...)

	config := options.Config
	if config.ID == "" {
		config.ID = id
	}

	return &Model{
		id:     id,
		config: config,
		client: &client,
	}, nil
}

// Config returns static metadata for this model.
func (m *Model) Config() kit.ModelConfig {
	return m.config
}

// Generate generates a response for the given request.
func (m *Model) Generate(ctx context.Context, request kit.ModelRequest) (kit.ModelResponse, error) {
	params := buildRequestParams(m.id, request)

	resp, err := m.client.Messages.New(ctx, params)
	if err != nil {
		return kit.ModelResponse{}, mapError(err)
	}

	return convertResponse(resp), nil
}

// Stream streams a response for the given request.
func (m *Model) Stream(ctx context.Context, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return newStream(ctx, m, request)
}

func buildRequestParams(modelID string, req kit.ModelRequest) anthropicsdk.MessageNewParams {
	params := anthropicsdk.MessageNewParams{
		Model:    modelID,
		System:   []anthropicsdk.TextBlockParam{{Text: req.Instructions}},
		Messages: convertRequestMessages(req.Messages),
		Tools:    convertRequestTools(req.Tools),
	}

	gc := req.Config
	if gc.Temperature != nil {
		params.Temperature = anthropicsdk.Float(float64(*gc.Temperature))
	}

	// Anthropic requires max_tokens >= 1
	// Keeping it set to 0 pupulates the prompt cache without generating response
	if gc.MaxOutputTokens != nil {
		params.MaxTokens = int64(*gc.MaxOutputTokens)
	} else {
		params.MaxTokens = defaultMaxTokens
	}

	if gc.Thinking != nil && *gc.Thinking != kit.ThinkingDisabled {
		params.Thinking = anthropicsdk.ThinkingConfigParamOfEnabled(thinkingBudgets[*gc.Thinking])
	}

	return params
}
