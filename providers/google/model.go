package google

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers"
)

var _ kit.Model = (*Model)(nil)

var thinkingLevels = map[kit.ThinkingLevel]genai.ThinkingLevel{
	kit.ThinkingLevelLow:    genai.ThinkingLevelLow,
	kit.ThinkingLevelMedium: genai.ThinkingLevelMedium,
	kit.ThinkingLevelHigh:   genai.ThinkingLevelHigh,
}

// Model implements the [kit.Model] interface using Google's Gemini API.
type Model struct {
	id     string
	config kit.ModelConfig
	client *genai.Client
}

// New returns a model for the given ID and options.
func New(id string, opts ...providers.ModelOption) (*Model, error) {
	options := providers.ModelOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	if options.CredentialsSource != nil {
		return nil, errors.New("google provider: credentials source is not supported")
	}

	headers := make(http.Header, len(options.Headers))
	for name, value := range options.Headers {
		headers.Set(name, value)
	}

	cc := &genai.ClientConfig{
		APIKey:  options.APIKey,
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			Headers: headers,
		},
	}

	if options.BaseURL != "" {
		cc.HTTPOptions.BaseURL = options.BaseURL
	}

	client, err := genai.NewClient(context.Background(), cc)
	if err != nil {
		return nil, err
	}

	config := options.Config
	if config.ID == "" {
		config.ID = id
	}

	return &Model{
		id:     id,
		config: config,
		client: client,
	}, nil
}

// Config returns static metadata for this model.
func (m *Model) Config() kit.ModelConfig {
	return m.config
}

// Generate generates a response for the given request.
func (m *Model) Generate(ctx context.Context, request kit.ModelRequest) (kit.ModelResponse, error) {
	contents, config := buildRequestParams(request)

	resp, err := m.client.Models.GenerateContent(ctx, m.id, contents, config)
	if err != nil {
		return kit.ModelResponse{}, mapError(err)
	}

	return convertResponse(resp), nil
}

// Stream streams a response for the given request.
func (m *Model) Stream(ctx context.Context, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return newStream(ctx, m, request)
}

func buildRequestParams(req kit.ModelRequest) ([]*genai.Content, *genai.GenerateContentConfig) {
	config := &genai.GenerateContentConfig{
		Tools: convertRequestTools(req.Tools),
	}

	if strings.TrimSpace(req.Instructions) != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: req.Instructions}},
		}
	}

	gc := req.Config
	if gc.Temperature != nil {
		config.Temperature = gc.Temperature
	}

	if gc.MaxOutputTokens != nil {
		config.MaxOutputTokens = *gc.MaxOutputTokens
	}

	if gc.Thinking != nil && *gc.Thinking != kit.ThinkingDisabled {
		config.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingLevel:   thinkingLevels[*gc.Thinking],
			IncludeThoughts: true,
		}
	}

	if req.OutputSchema != nil {
		config.ResponseMIMEType = "application/json"
		config.ResponseJsonSchema = req.OutputSchema.Schema
	}

	return convertRequestMessages(req.Messages), config
}
