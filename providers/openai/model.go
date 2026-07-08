package openai

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Model = (*Model)(nil)

type options struct {
	baseURL string
	config  kit.ModelConfig
}

// Option customizes the OpenAI provider.
type Option func(*options)

// WithBaseURL points the provider at an OpenAI-compatible endpoint.
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

// Model implements the [kit.Model] interface using OpenAI's response API.
type Model struct {
	id     string
	config kit.ModelConfig
	client *openai.Client
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

	client := openai.NewClient(clientOptions...)

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
	req := buildRequestParams(m.id, request)

	resp, err := m.client.Responses.New(ctx, req)
	if err != nil {
		return kit.ModelResponse{}, mapError(err)
	}

	return convertResponse(resp), nil
}

// Stream streams a response for the given request.
func (m *Model) Stream(ctx context.Context, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return newStream(ctx, m, request)
}

func buildRequestParams(modelID string, req kit.ModelRequest) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model:        modelID,
		Instructions: openai.String(req.Instructions),
		Tools:        convertRequestTools(req.Tools),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: convertRequestMessages(req.Messages),
		},
	}

	gc := req.Config
	if gc.Temperature != nil {
		params.Temperature = openai.Float(float64(*gc.Temperature))
	}

	if gc.MaxOutputTokens != nil {
		params.MaxOutputTokens = openai.Int(int64(*gc.MaxOutputTokens))
	}

	if gc.Thinking != nil && *gc.Thinking != kit.ThinkingDisabled {
		params.Reasoning = shared.ReasoningParam{
			Effort:  convertRequestReasoningEffort(*gc.Thinking),
			Summary: shared.ReasoningSummaryAuto,
		}
	}

	return params
}
