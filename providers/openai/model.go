package openai

import (
	"context"
	"net/http"

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
func New(id string, opts ...providers.ModelOption) (*Model, error) {
	options := providers.ModelOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	var clientOptions []option.RequestOption
	if options.BaseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(options.BaseURL))
	}

	if options.CredentialsSource == nil {
		if options.BearerToken != "" {
			clientOptions = append(clientOptions, option.WithAPIKey(options.BearerToken))
		} else {
			clientOptions = append(clientOptions, option.WithAPIKey(options.APIKey))
		}
	}

	for name, value := range options.Headers {
		clientOptions = append(clientOptions, option.WithHeader(name, value))
	}

	if options.CredentialsSource != nil {
		clientOptions = append(
			clientOptions,
			option.WithMiddleware(credentialsMiddleware(options.CredentialsSource)),
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

	return params
}

func credentialsMiddleware(source providers.CredentialsSource) option.Middleware {
	return func(request *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		headers, err := source.Headers(request.Context())
		if err != nil {
			return nil, err
		}

		request.Header.Del("Authorization")

		for name, values := range headers {
			request.Header.Del(name)

			for _, value := range values {
				request.Header.Add(name, value)
			}
		}

		return next(request)
	}
}
