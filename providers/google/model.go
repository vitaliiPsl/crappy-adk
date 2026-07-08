package google

import (
	"context"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

var _ kit.Model = (*Model)(nil)

var thinkingLevels = map[kit.ThinkingLevel]genai.ThinkingLevel{
	kit.ThinkingLevelLow:    genai.ThinkingLevelLow,
	kit.ThinkingLevelMedium: genai.ThinkingLevelMedium,
	kit.ThinkingLevelHigh:   genai.ThinkingLevelHigh,
}

type options struct {
	baseURL string
	config  kit.ModelConfig
}

// Option customizes the Google provider.
type Option func(*options)

// WithBaseURL points the provider at a Gemini-compatible endpoint.
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

// Model implements the [kit.Model] interface using Google's Gemini API.
type Model struct {
	id     string
	config kit.ModelConfig
	client *genai.Client
}

// New returns an authenticated model for the given modelID and apiKey.
func New(apiKey string, id string, opts ...Option) (*Model, error) {
	options := options{}
	for _, opt := range opts {
		opt(&options)
	}

	cc := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}

	if options.baseURL != "" {
		cc.HTTPOptions = genai.HTTPOptions{BaseURL: options.baseURL}
	}

	client, err := genai.NewClient(context.Background(), cc)
	if err != nil {
		return nil, err
	}

	config := options.config
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
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: req.Instructions}}},
		Tools:             convertRequestTools(req.Tools),
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

	return convertRequestMessages(req.Messages), config
}
