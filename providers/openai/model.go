package openai

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers"
)

var _ kit.Model = (*Model)(nil)

// Model implements the [kit.Model] interface using OpenAI's response API.
type Model struct {
	id     string
	config kit.ModelConfig
	client *openai.Client
}

// New returns a model for the given ID and options.
func New(id string, opts ...providers.ModelOption) (kit.Model, error) {
	options := providers.ModelOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	var clientOptions []option.RequestOption
	if options.BaseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(options.BaseURL))
	}

	if credentials := resolveCredentials(options); credentials != nil {
		clientOptions = append(
			clientOptions,
			option.WithMiddleware(credentialsMiddleware(credentials)),
		)
	}

	client := openai.NewClient(clientOptions...)

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
	return m.Stream(ctx, request).Result()
}

// Stream streams a response for the given request.
func (m *Model) Stream(ctx context.Context, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return newStream(ctx, m, request)
}

func buildRequestParams(modelID string, req kit.ModelRequest) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model:        modelID,
		Instructions: openai.String(req.Instructions),
		Store:        openai.Bool(false),
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

	if req.OutputSchema != nil {
		format := responses.ResponseFormatTextConfigParamOfJSONSchema(
			req.OutputSchema.Name,
			req.OutputSchema.Schema,
		)

		format.OfJSONSchema.Strict = openai.Bool(true)
		if req.OutputSchema.Description != "" {
			format.OfJSONSchema.Description = openai.String(req.OutputSchema.Description)
		}

		params.Text.Format = format
	}

	return params
}
