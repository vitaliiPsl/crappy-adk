package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

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

func convertRequestTools(tools []kit.Tool) []responses.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	result := make([]responses.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		def := tool.Definition()
		result = append(result, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        def.Name,
				Description: openai.String(def.Description),
				Parameters:  def.Schema,
			},
		})
	}

	return result
}

func convertRequestMessages(messages []kit.Message) responses.ResponseInputParam {
	reqMessages := make(responses.ResponseInputParam, 0, len(messages))
	for _, msg := range messages {
		reqMessages = append(reqMessages, convertRequestMessage(msg)...)
	}

	return reqMessages
}

func convertRequestMessage(msg kit.Message) responses.ResponseInputParam {
	switch msg.Role {
	case kit.RoleTool:
		return convertRequestToolMessage(msg)
	case kit.RoleModel:
		return convertAssistantRequestMessage(msg)
	default:
		return convertRequestUserMessage(msg)
	}
}

func convertRequestUserMessage(msg kit.Message) responses.ResponseInputParam {
	list := make(responses.ResponseInputMessageContentListParam, 0, len(msg.Content))
	for _, c := range msg.Content {
		item, ok := convertRequestContentListItem(c)
		if ok {
			list = append(list, item)
		}
	}

	return []responses.ResponseInputItemUnionParam{{
		OfMessage: &responses.EasyInputMessageParam{
			Role:    responses.EasyInputMessageRoleUser,
			Content: responses.EasyInputMessageContentUnionParam{OfInputItemContentList: list},
		},
	}}
}

func convertAssistantRequestMessage(msg kit.Message) responses.ResponseInputParam {
	items := make(responses.ResponseInputParam, 0, len(msg.Content))
	for _, c := range msg.Content {
		if item, ok := convertRequestContentItem(c, responses.EasyInputMessageRoleAssistant); ok {
			items = append(items, item)
		}
	}

	return items
}

func convertRequestToolMessage(msg kit.Message) responses.ResponseInputParam {
	items := make(responses.ResponseInputParam, 0, len(msg.Content))
	for _, c := range msg.Content {
		if item, ok := convertRequestContentItem(c, ""); ok {
			items = append(items, item)
		}
	}

	return items
}

func convertRequestContentItem(content kit.Content, role responses.EasyInputMessageRole) (responses.ResponseInputItemUnionParam, bool) {
	switch content.Type {
	case kit.ContentTypeText:
		if content.Text == nil {
			return responses.ResponseInputItemUnionParam{}, false
		}

		return responses.ResponseInputItemUnionParam{
			OfMessage: &responses.EasyInputMessageParam{
				Role:    role,
				Content: responses.EasyInputMessageContentUnionParam{OfString: openai.String(content.Text.Text)},
			},
		}, true
	case kit.ContentTypeThinking:
		if content.Thinking == nil {
			return responses.ResponseInputItemUnionParam{}, false
		}

		t := content.Thinking

		reasoning := responses.ResponseReasoningItemParam{
			ID: t.ID,
			Summary: []responses.ResponseReasoningItemSummaryParam{
				{Text: t.Text},
			},
		}
		if t.Signature != "" {
			reasoning.EncryptedContent = openai.String(t.Signature)
		}

		return responses.ResponseInputItemUnionParam{OfReasoning: &reasoning}, true
	case kit.ContentTypeImage, kit.ContentTypeAudio, kit.ContentTypeResource:
		item, ok := convertRequestContentListItem(content)
		if !ok {
			return responses.ResponseInputItemUnionParam{}, false
		}

		return responses.ResponseInputItemUnionParam{
			OfMessage: &responses.EasyInputMessageParam{
				Role:    role,
				Content: responses.EasyInputMessageContentUnionParam{OfInputItemContentList: responses.ResponseInputMessageContentListParam{item}},
			},
		}, true
	case kit.ContentTypeToolCall:
		if content.ToolCall == nil {
			return responses.ResponseInputItemUnionParam{}, false
		}

		tc := content.ToolCall

		argsJSON, err := json.Marshal(tc.Arguments)
		if err != nil {
			return responses.ResponseInputItemUnionParam{}, false
		}

		return responses.ResponseInputItemUnionParam{
			OfFunctionCall: &responses.ResponseFunctionToolCallParam{
				CallID:    tc.ID,
				Name:      tc.Name,
				Arguments: string(argsJSON),
			},
		}, true
	case kit.ContentTypeToolResult:
		if content.ToolResult == nil {
			return responses.ResponseInputItemUnionParam{}, false
		}

		tr := content.ToolResult

		output := tr.Output
		if output == "" {
			output = tr.Error
		}

		return responses.ResponseInputItemUnionParam{
			OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
				CallID: tr.Call.ID,
				Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
					OfString: openai.String(output),
				},
			},
		}, true
	case kit.ContentTypeSummary:
		if content.Summary == nil {
			return responses.ResponseInputItemUnionParam{}, false
		}

		return responses.ResponseInputItemUnionParam{
			OfMessage: &responses.EasyInputMessageParam{
				Role:    role,
				Content: responses.EasyInputMessageContentUnionParam{OfString: openai.String(content.Summary.Text)},
			},
		}, true
	}

	return responses.ResponseInputItemUnionParam{}, false
}

func convertRequestContentListItem(content kit.Content) (responses.ResponseInputContentUnionParam, bool) {
	switch content.Type {
	case kit.ContentTypeText:
		if content.Text == nil {
			return responses.ResponseInputContentUnionParam{}, false
		}

		return responses.ResponseInputContentParamOfInputText(content.Text.Text), true
	case kit.ContentTypeSummary:
		if content.Summary == nil {
			return responses.ResponseInputContentUnionParam{}, false
		}

		return responses.ResponseInputContentParamOfInputText(content.Summary.Text), true
	case kit.ContentTypeImage:
		image, ok := convertInputImage(content.Image)

		return responses.ResponseInputContentUnionParam{OfInputImage: image}, ok
	case kit.ContentTypeResource:
		if content.Resource == nil {
			return responses.ResponseInputContentUnionParam{}, false
		}

		if content.Resource.Text != "" {
			return responses.ResponseInputContentParamOfInputText(content.Resource.Text), true
		}

		if strings.HasPrefix(content.Resource.MIMEType, "image/") {
			image, ok := convertInputImage(&kit.Image{
				MIMEType: content.Resource.MIMEType,
				Data:     content.Resource.Blob,
				URI:      content.Resource.URI,
			})

			return responses.ResponseInputContentUnionParam{OfInputImage: image}, ok
		}

		file, ok := convertInputFile(content.Resource)

		return responses.ResponseInputContentUnionParam{OfInputFile: file}, ok
	case kit.ContentTypeAudio:
		text, ok := kit.ContentTextFallback(content)
		if !ok {
			return responses.ResponseInputContentUnionParam{}, false
		}

		return responses.ResponseInputContentParamOfInputText(text), true
	default:
		text, ok := kit.ContentTextFallback(content)
		if !ok {
			return responses.ResponseInputContentUnionParam{}, false
		}

		return responses.ResponseInputContentParamOfInputText(text), true
	}
}

func convertInputImage(image *kit.Image) (*responses.ResponseInputImageParam, bool) {
	if image == nil {
		return nil, false
	}

	out := &responses.ResponseInputImageParam{Detail: responses.ResponseInputImageDetailAuto}
	switch {
	case len(image.Data) > 0:
		out.ImageURL = openai.String(dataURL(image.MIMEType, image.Data))
	case image.URI != "":
		out.ImageURL = openai.String(image.URI)
	default:
		return nil, false
	}

	return out, true
}

func convertInputFile(resource *kit.Resource) (*responses.ResponseInputFileParam, bool) {
	if resource == nil {
		return nil, false
	}

	out := &responses.ResponseInputFileParam{
		Detail:   responses.ResponseInputFileDetailLow,
		Filename: openai.String(resourceFilename(resource)),
	}

	switch {
	case len(resource.Blob) > 0:
		out.FileData = openai.String(base64.StdEncoding.EncodeToString(resource.Blob))
	case resource.URI != "":
		out.FileURL = openai.String(resource.URI)
	default:
		return nil, false
	}

	return out, true
}

func dataURL(mimeType string, data []byte) string {
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func resourceFilename(resource *kit.Resource) string {
	switch {
	case resource.Name != "":
		return resource.Name
	case resource.Title != "":
		return resource.Title
	case resource.URI != "":
		parts := strings.Split(strings.TrimRight(resource.URI, "/"), "/")
		if len(parts) > 0 && parts[len(parts)-1] != "" {
			return parts[len(parts)-1]
		}
	}

	return "resource"
}

func convertRequestReasoningEffort(level kit.ThinkingLevel) shared.ReasoningEffort {
	switch level {
	case kit.ThinkingLevelLow:
		return shared.ReasoningEffortLow
	case kit.ThinkingLevelMedium:
		return shared.ReasoningEffortMedium
	case kit.ThinkingLevelHigh:
		return shared.ReasoningEffortHigh
	default:
		return shared.ReasoningEffortMedium
	}
}

func convertResponse(resp *responses.Response) kit.ModelResponse {
	content := make([]kit.Content, 0, len(resp.Output))

	for _, item := range resp.Output {
		content = append(content, convertResponseContent(item)...)
	}

	return kit.ModelResponse{
		Message:      kit.NewModelMessage(content...),
		FinishReason: convertResponseFinishReason(resp),
		Usage:        convertResponseUsage(resp),
	}
}

func convertResponseContent(item responses.ResponseOutputItemUnion) []kit.Content {
	switch item := item.AsAny().(type) {
	case responses.ResponseOutputMessage:
		var parts []kit.Content
		for _, part := range item.Content {
			if text := part.AsOutputText(); text.Text != "" {
				parts = append(parts, kit.NewTextContent(text.Text))
			}
		}

		return parts

	case responses.ResponseReasoningItem:
		var reasoningText strings.Builder
		for _, summary := range item.Summary {
			reasoningText.WriteString(summary.Text)
		}

		return []kit.Content{kit.NewThinkingContent(item.ID, reasoningText.String(), item.EncryptedContent)}

	case responses.ResponseFunctionToolCall:
		if tc, ok := convertResponseToolCall(item); ok {
			return []kit.Content{kit.NewToolCallContent(tc)}
		}
	}

	return nil
}

func convertResponseToolCall(item responses.ResponseFunctionToolCall) (kit.ToolCall, bool) {
	var args map[string]any
	if s := item.Arguments; s != "" {
		if err := json.Unmarshal([]byte(s), &args); err != nil {
			return kit.ToolCall{}, false
		}
	}

	return kit.ToolCall{
		ID:        item.CallID,
		Name:      item.Name,
		Arguments: args,
	}, true
}

func convertResponseFinishReason(resp *responses.Response) kit.FinishReason {
	switch resp.Status {
	case responses.ResponseStatusCompleted:
		for _, item := range resp.Output {
			if _, ok := item.AsAny().(responses.ResponseFunctionToolCall); ok {
				return kit.FinishReasonToolCall
			}
		}

		return kit.FinishReasonStop
	case responses.ResponseStatusIncomplete:
		switch resp.IncompleteDetails.Reason {
		case "max_output_tokens":
			return kit.FinishReasonMaxTokens
		case "content_filter":
			return kit.FinishReasonSafety
		}

		return kit.FinishReasonUnknown
	default:
		return kit.FinishReasonUnknown
	}
}

func convertResponseUsage(resp *responses.Response) kit.Usage {
	return kit.Usage{
		InputTokens:     resp.Usage.InputTokens,
		OutputTokens:    resp.Usage.OutputTokens,
		CacheReadTokens: resp.Usage.InputTokensDetails.CachedTokens,
		ReasoningTokens: resp.Usage.OutputTokensDetails.ReasoningTokens,
	}
}
