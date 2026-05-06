package anthropic

import (
	"context"
	"encoding/json"

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
}

// Option customizes the Anthropic provider.
type Option func(*options)

// WithBaseURL points the provider at an Anthropic-compatible endpoint.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// Model implements the [kit.Model] interface using Anthropic's messages API.
type Model struct {
	id     string
	client *anthropicsdk.Client
}

// New returns an authenticated model for the given modelID and apiKey.
func New(apiKey string, id string, opts ...Option) *Model {
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

	return &Model{
		id:     id,
		client: &client,
	}
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

func convertRequestTools(tools []kit.Tool) []anthropicsdk.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	result := make([]anthropicsdk.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		def := tool.Definition()
		result = append(result, anthropicsdk.ToolUnionParam{
			OfTool: &anthropicsdk.ToolParam{
				Name:        def.Name,
				Description: anthropicsdk.String(def.Description),
				InputSchema: convertToolInputSchema(def.Schema),
			},
		})
	}

	return result
}

func convertToolInputSchema(schema map[string]any) anthropicsdk.ToolInputSchemaParam {
	result := anthropicsdk.ToolInputSchemaParam{}

	if props, ok := schema["properties"]; ok {
		result.Properties = props
	}

	if req, ok := schema["required"]; ok {
		if reqSlice, ok := req.([]any); ok {
			required := make([]string, 0, len(reqSlice))
			for _, r := range reqSlice {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}

			result.Required = required
		}
	}

	extras := make(map[string]any)
	for k, v := range schema {
		if k != "type" && k != "properties" && k != "required" {
			extras[k] = v
		}
	}

	if len(extras) > 0 {
		result.ExtraFields = extras
	}

	return result
}

func convertRequestMessages(messages []kit.Message) []anthropicsdk.MessageParam {
	reqMessages := make([]anthropicsdk.MessageParam, 0, len(messages))
	for _, msg := range messages {
		reqMessages = append(reqMessages, convertRequestMessage(msg)...)
	}

	return reqMessages
}

func convertRequestMessage(msg kit.Message) []anthropicsdk.MessageParam {
	items := convertRequestContentItems(msg.Content)

	switch msg.Role {
	case kit.RoleModel:
		return []anthropicsdk.MessageParam{anthropicsdk.NewAssistantMessage(items...)}
	default:
		return []anthropicsdk.MessageParam{anthropicsdk.NewUserMessage(items...)}
	}
}

func convertRequestContentItems(content []kit.Content) []anthropicsdk.ContentBlockParamUnion {
	items := make([]anthropicsdk.ContentBlockParamUnion, 0, len(content))
	for _, c := range content {
		if item, ok := convertRequestContentItem(c); ok {
			items = append(items, item)
		}
	}

	return items
}

func convertRequestContentItem(content kit.Content) (anthropicsdk.ContentBlockParamUnion, bool) {
	switch content.Type {
	case kit.ContentTypeText:
		if content.Text == nil {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		return anthropicsdk.NewTextBlock(content.Text.Text), true
	case kit.ContentTypeThinking:
		if content.Thinking == nil {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		t := content.Thinking

		return anthropicsdk.NewThinkingBlock(t.Signature, t.Text), true
	case kit.ContentTypeToolCall:
		if content.ToolCall == nil {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		tc := content.ToolCall

		return anthropicsdk.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name), true
	case kit.ContentTypeToolResult:
		if content.ToolResult == nil {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		tr := content.ToolResult

		output := tr.Output
		if output == "" {
			output = tr.Error
		}

		return anthropicsdk.NewToolResultBlock(tr.Call.ID, output, tr.Error != ""), true
	case kit.ContentTypeSummary:
		if content.Summary == nil {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		return anthropicsdk.NewTextBlock(content.Summary.Text), true
	}

	return anthropicsdk.ContentBlockParamUnion{}, false
}

func convertResponse(resp *anthropicsdk.Message) kit.ModelResponse {
	content := make([]kit.Content, 0, len(resp.Content))

	for _, block := range resp.Content {
		content = append(content, convertResponseContent(block)...)
	}

	return kit.ModelResponse{
		Message:      kit.NewModelMessage(content),
		FinishReason: convertResponseFinishReason(resp),
		Usage:        convertResponseUsage(resp),
	}
}

func convertResponseContent(block anthropicsdk.ContentBlockUnion) []kit.Content {
	switch block.Type {
	case "text":
		return []kit.Content{kit.NewTextContent(block.Text)}

	case "thinking":
		return []kit.Content{kit.NewThinkingContent("", block.Thinking, block.Signature)}

	case "tool_use":
		if tc, ok := convertResponseToolCall(block); ok {
			return []kit.Content{kit.NewToolCallContent(tc)}
		}
	}

	return nil
}

func convertResponseToolCall(block anthropicsdk.ContentBlockUnion) (kit.ToolCall, bool) {
	var args map[string]any
	if err := json.Unmarshal(block.Input, &args); err != nil {
		return kit.ToolCall{}, false
	}

	return kit.ToolCall{
		ID:        block.ID,
		Name:      block.Name,
		Arguments: args,
	}, true
}

func convertResponseFinishReason(resp *anthropicsdk.Message) kit.FinishReason {
	switch resp.StopReason {
	case anthropicsdk.StopReasonEndTurn:
		return kit.FinishReasonStop
	case anthropicsdk.StopReasonMaxTokens:
		return kit.FinishReasonMaxTokens
	case anthropicsdk.StopReasonToolUse:
		return kit.FinishReasonToolCall
	case anthropicsdk.StopReasonRefusal:
		return kit.FinishReasonSafety
	default:
		return kit.FinishReasonUnknown
	}
}

func convertResponseUsage(resp *anthropicsdk.Message) kit.Usage {
	return kit.Usage{
		InputTokens:      int32(resp.Usage.InputTokens),
		OutputTokens:     int32(resp.Usage.OutputTokens),
		CacheReadTokens:  int32(resp.Usage.CacheReadInputTokens),
		CacheWriteTokens: int32(resp.Usage.CacheCreationInputTokens),
	}
}
