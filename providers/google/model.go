package google

import (
	"context"
	"encoding/base64"
	"strings"

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

func convertRequestTools(tools []kit.Tool) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	declarations := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		def := tool.Definition()
		declarations = append(declarations, &genai.FunctionDeclaration{
			Name:                 def.Name,
			Description:          def.Description,
			ParametersJsonSchema: def.Schema,
		})
	}

	return []*genai.Tool{{FunctionDeclarations: declarations}}
}

func convertRequestMessages(messages []kit.Message) []*genai.Content {
	contents := make([]*genai.Content, 0, len(messages))
	for _, msg := range messages {
		contents = append(contents, convertRequestMessage(msg))
	}

	return contents
}

func convertRequestMessage(msg kit.Message) *genai.Content {
	parts := convertRequestContentItems(msg.Content)

	switch msg.Role {
	case kit.RoleModel:
		return &genai.Content{Role: genai.RoleModel, Parts: parts}
	default:
		return &genai.Content{Role: genai.RoleUser, Parts: parts}
	}
}

func convertRequestContentItems(content []kit.Content) []*genai.Part {
	parts := make([]*genai.Part, 0, len(content))
	for _, c := range content {
		if part, ok := convertRequestContentItem(c); ok {
			parts = append(parts, part)
		}
	}

	return parts
}

func convertRequestContentItem(content kit.Content) (*genai.Part, bool) {
	switch content.Type {
	case kit.ContentTypeText:
		if content.Text == nil {
			return nil, false
		}

		return genai.NewPartFromText(content.Text.Text), true
	case kit.ContentTypeThinking:
		if content.Thinking == nil {
			return nil, false
		}

		t := content.Thinking

		return &genai.Part{
			Thought:          true,
			Text:             t.Text,
			ThoughtSignature: decodeSignature(t.Signature),
		}, true
	case kit.ContentTypeImage:
		return convertImageContent(content.Image)
	case kit.ContentTypeAudio:
		return convertAudioContent(content.Audio)
	case kit.ContentTypeResource:
		return convertResourceContent(content.Resource)
	case kit.ContentTypeToolCall:
		if content.ToolCall == nil {
			return nil, false
		}

		tc := content.ToolCall

		part := genai.NewPartFromFunctionCall(tc.Name, tc.Arguments)
		part.FunctionCall.ID = tc.ID
		part.ThoughtSignature = decodeSignature(tc.Signature)

		return part, true
	case kit.ContentTypeToolResult:
		if content.ToolResult == nil {
			return nil, false
		}

		tr := content.ToolResult

		response := map[string]any{}
		if output := kit.ContentsText(tr.Output.Content); output != "" {
			response["output"] = output
		}
		if tr.Output.Structured != nil {
			response["structured"] = tr.Output.Structured
		}

		if tr.Error != "" {
			response["error"] = tr.Error
		}
		if len(response) == 0 {
			response["output"] = ""
		}

		parts := convertFunctionResponseParts(tr.Output.Content)

		var part *genai.Part
		if len(parts) > 0 {
			part = genai.NewPartFromFunctionResponseWithParts(tr.Call.Name, response, parts)
		} else {
			part = genai.NewPartFromFunctionResponse(tr.Call.Name, response)
		}

		part.FunctionResponse.ID = tr.Call.ID

		return part, true
	case kit.ContentTypeSummary:
		if content.Summary == nil {
			return nil, false
		}

		return genai.NewPartFromText(content.Summary.Text), true
	}

	return nil, false
}

func convertImageContent(image *kit.Image) (*genai.Part, bool) {
	if image == nil {
		return nil, false
	}

	if len(image.Data) > 0 {
		return genai.NewPartFromBytes(image.Data, image.MIMEType), true
	}

	if image.URI != "" {
		return genai.NewPartFromURI(image.URI, image.MIMEType), true
	}

	return fallbackPart(kit.Content{Type: kit.ContentTypeImage, Image: image})
}

func convertAudioContent(audio *kit.Audio) (*genai.Part, bool) {
	if audio == nil {
		return nil, false
	}

	if len(audio.Data) > 0 {
		return genai.NewPartFromBytes(audio.Data, audio.MIMEType), true
	}

	if audio.URI != "" {
		return genai.NewPartFromURI(audio.URI, audio.MIMEType), true
	}

	return fallbackPart(kit.Content{Type: kit.ContentTypeAudio, Audio: audio})
}

func convertResourceContent(resource *kit.Resource) (*genai.Part, bool) {
	if resource == nil {
		return nil, false
	}

	if resource.Text != "" {
		return genai.NewPartFromText(resource.Text), true
	}

	if len(resource.Blob) > 0 {
		return genai.NewPartFromBytes(resource.Blob, resource.MIMEType), true
	}

	if resource.URI != "" {
		return genai.NewPartFromURI(resource.URI, resource.MIMEType), true
	}

	return fallbackPart(kit.Content{Type: kit.ContentTypeResource, Resource: resource})
}

func fallbackPart(content kit.Content) (*genai.Part, bool) {
	text, ok := kit.ContentText(content)
	if !ok {
		return nil, false
	}

	return genai.NewPartFromText(text), true
}

func convertFunctionResponseParts(content []kit.Content) []*genai.FunctionResponsePart {
	parts := make([]*genai.FunctionResponsePart, 0, len(content))
	for _, item := range content {
		if part, ok := convertFunctionResponsePart(item); ok {
			parts = append(parts, part)
		}
	}

	return parts
}

func convertFunctionResponsePart(content kit.Content) (*genai.FunctionResponsePart, bool) {
	switch content.Type {
	case kit.ContentTypeImage:
		if content.Image == nil {
			return nil, false
		}

		if len(content.Image.Data) > 0 {
			return genai.NewFunctionResponsePartFromBytes(content.Image.Data, content.Image.MIMEType), true
		}

		if content.Image.URI != "" {
			return genai.NewFunctionResponsePartFromURI(content.Image.URI, content.Image.MIMEType), true
		}
	case kit.ContentTypeAudio:
		if content.Audio == nil {
			return nil, false
		}

		if len(content.Audio.Data) > 0 {
			return genai.NewFunctionResponsePartFromBytes(content.Audio.Data, content.Audio.MIMEType), true
		}

		if content.Audio.URI != "" {
			return genai.NewFunctionResponsePartFromURI(content.Audio.URI, content.Audio.MIMEType), true
		}
	case kit.ContentTypeResource:
		if content.Resource == nil {
			return nil, false
		}

		if len(content.Resource.Blob) > 0 {
			return genai.NewFunctionResponsePartFromBytes(content.Resource.Blob, content.Resource.MIMEType), true
		}

		if content.Resource.URI != "" {
			return genai.NewFunctionResponsePartFromURI(content.Resource.URI, content.Resource.MIMEType), true
		}
	}

	return nil, false
}

func convertResponse(resp *genai.GenerateContentResponse) kit.ModelResponse {
	if len(resp.Candidates) == 0 {
		return kit.ModelResponse{}
	}

	candidate := resp.Candidates[0]
	content := make([]kit.Content, 0)

	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			content = append(content, convertResponseContent(part)...)
		}
	}

	return kit.ModelResponse{
		Message:      kit.NewModelMessage(content...),
		FinishReason: convertResponseFinishReason(candidate),
		Usage:        convertResponseUsage(resp),
	}
}

func convertResponseContent(part *genai.Part) []kit.Content {
	if part.Thought {
		return []kit.Content{kit.NewThinkingContent("", part.Text, encodeSignature(part.ThoughtSignature))}
	}

	if part.Text != "" {
		return []kit.Content{kit.NewTextContent(part.Text)}
	}

	if part.InlineData != nil {
		return []kit.Content{convertResponseBlob(part.InlineData)}
	}

	if part.FileData != nil {
		return []kit.Content{kit.NewResourceContent(kit.Resource{
			URI:      part.FileData.FileURI,
			MIMEType: part.FileData.MIMEType,
		})}
	}

	if part.FunctionCall != nil {
		if tc, ok := convertResponseToolCall(part); ok {
			return []kit.Content{kit.NewToolCallContent(tc)}
		}
	}

	return nil
}

func convertResponseBlob(blob *genai.Blob) kit.Content {
	if blob == nil {
		return kit.Content{}
	}

	switch {
	case strings.HasPrefix(blob.MIMEType, "image/"):
		return kit.NewImageContent(blob.MIMEType, blob.Data)
	case strings.HasPrefix(blob.MIMEType, "audio/"):
		return kit.NewAudioContent(blob.MIMEType, blob.Data)
	default:
		return kit.NewResourceContent(kit.Resource{
			MIMEType: blob.MIMEType,
			Blob:     blob.Data,
		})
	}
}

func convertResponseToolCall(part *genai.Part) (kit.ToolCall, bool) {
	if part == nil || part.FunctionCall == nil {
		return kit.ToolCall{}, false
	}

	fc := part.FunctionCall

	return kit.ToolCall{
		ID:        fc.ID,
		Name:      fc.Name,
		Arguments: fc.Args,
		Signature: encodeSignature(part.ThoughtSignature),
	}, true
}

func encodeSignature(sig []byte) string {
	if len(sig) == 0 {
		return ""
	}

	return base64.StdEncoding.EncodeToString(sig)
}

func decodeSignature(sig string) []byte {
	if sig == "" {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(sig)
	if err == nil {
		return decoded
	}

	return []byte(sig)
}

func convertResponseFinishReason(candidate *genai.Candidate) kit.FinishReason {
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				return kit.FinishReasonToolCall
			}
		}
	}

	switch candidate.FinishReason {
	case genai.FinishReasonStop:
		return kit.FinishReasonStop
	case genai.FinishReasonMaxTokens:
		return kit.FinishReasonMaxTokens
	case genai.FinishReasonSafety:
		return kit.FinishReasonSafety
	default:
		return kit.FinishReasonUnknown
	}
}

func convertResponseUsage(resp *genai.GenerateContentResponse) kit.Usage {
	if resp.UsageMetadata == nil {
		return kit.Usage{}
	}

	return kit.Usage{
		InputTokens:     int64(resp.UsageMetadata.PromptTokenCount),
		OutputTokens:    int64(resp.UsageMetadata.CandidatesTokenCount),
		CacheReadTokens: int64(resp.UsageMetadata.CachedContentTokenCount),
		ReasoningTokens: int64(resp.UsageMetadata.ThoughtsTokenCount),
	}
}
