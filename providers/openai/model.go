package openai

import (
	"context"
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
}

// Option customizes the OpenAI provider.
type Option func(*options)

// WithBaseURL points the provider at an OpenAI-compatible endpoint.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// Model implements the [kit.Model] interface using OpenAI's response API.
type Model struct {
	id     string
	client *openai.Client
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

	client := openai.NewClient(clientOptions...)

	return &Model{
		id:     id,
		client: &client,
	}
}

// Generate generates a response for the given request.
func (m *Model) Generate(ctx context.Context, request kit.ModelRequest) (kit.ModelResponse, error) {
	req, err := buildRequestParams(m.id, request)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	resp, err := m.client.Responses.New(ctx, req)
	if err != nil {
		return kit.ModelResponse{}, mapError(err)
	}

	return convertResponse(resp), nil
}

func buildRequestParams(modelID string, req kit.ModelRequest) (responses.ResponseNewParams, error) {
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

	return params, nil
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
		if c.Type == kit.ContentTypeText && c.Text != nil {
			list = append(list, responses.ResponseInputContentParamOfInputText(c.Text.Text))
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
				Role: role,
				Content: responses.EasyInputMessageContentUnionParam{
					OfInputItemContentList: responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(content.Text.Text),
					},
				},
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
	}

	return responses.ResponseInputItemUnionParam{}, false
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
		Message:      kit.NewModelMessage(content),
		FinishReason: convertResponseFinishReason(resp),
		Usage:        convertResponseUsage(resp),
	}
}

func convertResponseContent(item responses.ResponseOutputItemUnion) []kit.Content {
	switch item.Type {
	case "message":
		msg := item.AsMessage()

		var parts []kit.Content
		for _, part := range msg.Content {
			if text := part.AsOutputText(); text.Text != "" {
				parts = append(parts, kit.NewTextContent(text.Text))
			}
		}

		return parts

	case "reasoning":
		reasoning := item.AsReasoning()

		var reasoningText strings.Builder
		for _, summary := range reasoning.Summary {
			reasoningText.WriteString(summary.Text)
		}

		return []kit.Content{kit.NewThinkingContent(reasoning.ID, reasoningText.String(), reasoning.EncryptedContent)}

	case "function_call":
		if tc, ok := convertResponseToolCall(item); ok {
			return []kit.Content{kit.NewToolCallContent(tc)}
		}
	}

	return nil
}

func convertResponseToolCall(item responses.ResponseOutputItemUnion) (kit.ToolCall, bool) {
	var args map[string]any
	if s := item.Arguments.OfString; s != "" {
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
			if item.Type == "function_call" {
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
		InputTokens:     int32(resp.Usage.InputTokens),
		OutputTokens:    int32(resp.Usage.OutputTokens),
		CacheReadTokens: int32(resp.Usage.InputTokensDetails.CachedTokens),
		ReasoningTokens: int32(resp.Usage.OutputTokensDetails.ReasoningTokens),
	}
}
