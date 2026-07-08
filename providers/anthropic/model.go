package anthropic

import (
	"context"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/vitaliiPsl/crappy-adk/kit"
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

type options struct {
	baseURL string
	config  kit.ModelConfig
}

// Option customizes the Anthropic provider.
type Option func(*options)

// WithBaseURL points the provider at an Anthropic-compatible endpoint.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// WithModelConfig overrides the static metadata returned by [Model.Config].
func WithModelConfig(config kit.ModelConfig) Option {
	return func(o *options) {
		o.config = config
	}
}

// Model implements the [kit.Model] interface using Anthropic's messages API.
type Model struct {
	id     string
	config kit.ModelConfig
	client *anthropicsdk.Client
}

// New returns an authenticated model for the given modelID and apiKey.
func New(apiKey string, id string, opts ...Option) (*Model, error) {
	options := options{}
	for _, opt := range opts {
		opt(&options)
	}

	clientOptions := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if options.baseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(options.baseURL))
	}

	client := anthropicsdk.NewClient(clientOptions...)

	config := options.config
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
