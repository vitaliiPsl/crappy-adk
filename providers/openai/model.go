package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/vitaliiPsl/crappy-adk/kit"
	xstream "github.com/vitaliiPsl/crappy-adk/x/stream"
	"github.com/vitaliiPsl/crappy-adk/x/structuredoutput"
)

// model implements [kit.Model] for an OpenAI model via the Responses API.
type model struct {
	client *openaisdk.Client
	config kit.ModelConfig
}

var _ kit.Model = (*model)(nil)

func (m *model) Config() kit.ModelConfig {
	return m.config
}

func (m *model) Generate(ctx context.Context, req kit.ModelRequest) (kit.ModelResponse, error) {
	params, err := buildParams(req, m.config.ID)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	resp, err := m.client.Responses.New(ctx, params)
	if err != nil {
		return kit.ModelResponse{}, convertError(err)
	}

	out := convertResponse(resp)

	out.StructuredOutput, err = structuredoutput.Validate(out.Message.Text(), req.ResponseSchema)
	if err != nil {
		return kit.ModelResponse{}, &kit.LLMError{
			Kind:      kit.ErrStructuredOutput,
			Message:   err.Error(),
			Retryable: false,
			Provider:  ProviderID,
			Cause:     err,
		}
	}

	return out, nil
}

func (m *model) GenerateStream(ctx context.Context, req kit.ModelRequest) (*xstream.Stream[kit.Event, kit.ModelResponse], error) {
	params, err := buildParams(req, m.config.ID)
	if err != nil {
		return nil, err
	}

	stream := m.client.Responses.NewStreaming(ctx, params)

	return xstream.New(func(emit *xstream.Emitter[kit.Event]) (kit.ModelResponse, error) {
		defer func() { _ = stream.Close() }()

		var (
			response           *responses.Response
			thinkingStarted    bool
			contentPartStarted bool
		)

		for stream.Next() {
			event := stream.Current()

			switch e := event.AsAny().(type) {
			case responses.ResponseTextDeltaEvent:
				if !contentPartStarted {
					contentPartStarted = true

					if err := emit.Emit(kit.NewContentPartStartedEvent(kit.ContentTypeText)); err != nil {
						return kit.ModelResponse{}, nil
					}
				}

				if err := emit.Emit(kit.NewContentPartDeltaEvent(kit.ContentTypeText, e.Delta)); err != nil {
					return kit.ModelResponse{}, nil
				}

			case responses.ResponseReasoningTextDeltaEvent:
				if !thinkingStarted {
					thinkingStarted = true

					if err := emit.Emit(kit.NewContentPartStartedEvent(kit.ContentTypeThinking)); err != nil {
						return kit.ModelResponse{}, nil
					}
				}

				if err := emit.Emit(kit.NewContentPartDeltaEvent(kit.ContentTypeThinking, e.Delta)); err != nil {
					return kit.ModelResponse{}, nil
				}

			case responses.ResponseOutputItemDoneEvent:
				if e.Item.Type != "function_call" {
					continue
				}

				tc, err := convertFunctionCall(e.Item)
				if err != nil {
					return kit.ModelResponse{}, err
				}

				toolPart := kit.NewToolCallPart(tc)

				if err := emit.Emit(kit.NewContentPartStartedEvent(kit.ContentTypeToolCall)); err != nil {
					return kit.ModelResponse{}, nil
				}

				if err := emit.Emit(kit.NewContentPartDoneEvent(toolPart)); err != nil {
					return kit.ModelResponse{}, nil
				}

			case responses.ResponseCompletedEvent:
				response = &e.Response
			}
		}

		if err := stream.Err(); err != nil {
			return kit.ModelResponse{}, convertError(err)
		}

		if response == nil {
			return kit.ModelResponse{}, nil
		}

		resp := convertResponse(response)

		resp.StructuredOutput, err = structuredoutput.Validate(resp.Message.Text(), req.ResponseSchema)
		if err != nil {
			return kit.ModelResponse{}, &kit.LLMError{
				Kind:      kit.ErrStructuredOutput,
				Message:   err.Error(),
				Retryable: false,
				Provider:  ProviderID,
				Cause:     err,
			}
		}

		for _, part := range resp.Message.Content {
			if part.Type == kit.ContentTypeRedactedThinking || part.Type == kit.ContentTypeToolCall {
				continue
			}

			if err := emit.Emit(kit.NewContentPartDoneEvent(part)); err != nil {
				return resp, nil
			}
		}

		return resp, nil
	}), nil
}

func buildParams(req kit.ModelRequest, modelID string) (responses.ResponseNewParams, error) {
	params := responses.ResponseNewParams{
		Model: modelID,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: convertMessages(req.Messages),
		},
		Tools: convertTools(req.Tools),
	}

	if req.ResponseSchema != nil {
		schema, err := structuredoutput.SchemaMap(req.ResponseSchema)
		if err != nil {
			return responses.ResponseNewParams{}, fmt.Errorf("openai: schema map: %w", err)
		}

		params.Text = responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("structured_output", schema),
		}
	}

	if req.Instruction != "" {
		params.Instructions = openaisdk.String(req.Instruction)
	}

	gc := req.Config
	if gc.Temperature != nil {
		params.Temperature = openaisdk.Float(float64(*gc.Temperature))
	}

	if gc.TopP != nil {
		params.TopP = openaisdk.Float(float64(*gc.TopP))
	}

	if gc.MaxOutputTokens != nil {
		params.MaxOutputTokens = openaisdk.Int(int64(*gc.MaxOutputTokens))
	}

	if gc.Thinking != kit.ThinkingDisabled {
		params.Reasoning = shared.ReasoningParam{
			Effort:  reasoningEffort(gc.Thinking),
			Summary: shared.ReasoningSummaryAuto,
		}
	}

	return params, nil
}

func reasoningEffort(level kit.ThinkingLevel) shared.ReasoningEffort {
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

func convertMessages(msgs []kit.Message) responses.ResponseInputParam {
	result := make(responses.ResponseInputParam, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, convertMessage(msg)...)
	}

	return result
}

func convertMessage(msg kit.Message) []responses.ResponseInputItemUnionParam {
	switch msg.Role {
	case kit.MessageRoleUser:
		return convertUserMessage(msg)

	case kit.MessageRoleAssistant:
		return convertAssistantMessage(msg)

	case kit.MessageRoleTool:
		return convertToolMessage(msg)

	default:
		return []responses.ResponseInputItemUnionParam{{
			OfMessage: &responses.EasyInputMessageParam{
				Role:    responses.EasyInputMessageRoleUser,
				Content: responses.EasyInputMessageContentUnionParam{OfString: openaisdk.String(msg.Text())},
			},
		}}
	}
}

func convertUserMessage(msg kit.Message) []responses.ResponseInputItemUnionParam {
	return []responses.ResponseInputItemUnionParam{{
		OfMessage: &responses.EasyInputMessageParam{
			Role:    responses.EasyInputMessageRoleUser,
			Content: convertUserContent(msg.Content),
		},
	}}
}

func convertAssistantMessage(msg kit.Message) []responses.ResponseInputItemUnionParam {
	items := make([]responses.ResponseInputItemUnionParam, 0, len(msg.Content)+len(msg.ToolCalls()))
	seenToolCalls := make(map[string]struct{})

	for _, part := range msg.Content {
		if call, ok := part.ToolCallValue(); ok {
			seenToolCalls[call.ID] = struct{}{}

			items = append(items, convertToolCallPart(part))

			continue
		}

		if item, ok := convertAssistantContentPart(part); ok {
			items = append(items, item)
		}
	}

	for _, tc := range msg.ToolCalls() {
		if _, ok := seenToolCalls[tc.ID]; ok {
			continue
		}

		args, _ := json.Marshal(tc.Arguments)
		items = append(items, responses.ResponseInputItemUnionParam{
			OfFunctionCall: &responses.ResponseFunctionToolCallParam{
				CallID:    tc.ID,
				Name:      tc.Name,
				Arguments: string(args),
			},
		})
	}

	return items
}

func convertToolMessage(msg kit.Message) []responses.ResponseInputItemUnionParam {
	callID := ""

	output := msg.Text()
	if part, ok := msg.ToolResult(); ok {
		callID = part.ID
		output = part.Text
	}

	return []responses.ResponseInputItemUnionParam{{
		OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
			CallID: callID,
			Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
				OfString: openaisdk.String(output),
			},
		},
	}}
}

func convertToolCallPart(part kit.ContentPart) responses.ResponseInputItemUnionParam {
	call, _ := part.ToolCallValue()
	args, _ := json.Marshal(call.Arguments)

	return responses.ResponseInputItemUnionParam{
		OfFunctionCall: &responses.ResponseFunctionToolCallParam{
			CallID:    call.ID,
			Name:      call.Name,
			Arguments: string(args),
		},
	}
}

func convertUserContent(parts []kit.ContentPart) responses.EasyInputMessageContentUnionParam {
	if len(parts) == 1 && (parts[0].Type == kit.ContentTypeText || parts[0].Type == kit.ContentTypeSummary) {
		return responses.EasyInputMessageContentUnionParam{OfString: openaisdk.String(parts[0].TextValue())}
	}

	list := make(responses.ResponseInputMessageContentListParam, 0, len(parts))
	for _, p := range parts {
		if item, ok := convertUserContentPart(p); ok {
			list = append(list, item)
		}
	}

	return responses.EasyInputMessageContentUnionParam{OfInputItemContentList: list}
}

func convertUserContentPart(p kit.ContentPart) (responses.ResponseInputContentUnionParam, bool) {
	switch p.Type {
	case kit.ContentTypeText, kit.ContentTypeSummary:
		return responses.ResponseInputContentUnionParam{
			OfInputText: &responses.ResponseInputTextParam{Text: p.TextValue()},
		}, true
	case kit.ContentTypeImage:
		blob, ok := p.BlobValue()
		if !ok {
			return responses.ResponseInputContentUnionParam{}, false
		}

		img := &responses.ResponseInputImageParam{Detail: responses.ResponseInputImageDetailAuto}
		if blob.URL != "" {
			img.ImageURL = openaisdk.String(blob.URL)
		} else if len(blob.Data) > 0 {
			img.ImageURL = openaisdk.String("data:" + blob.MediaType + ";base64," + base64.StdEncoding.EncodeToString(blob.Data))
		} else {
			return responses.ResponseInputContentUnionParam{}, false
		}

		return responses.ResponseInputContentUnionParam{OfInputImage: img}, true
	case kit.ContentTypeDocument:
		blob, ok := p.BlobValue()
		if !ok {
			return responses.ResponseInputContentUnionParam{}, false
		}

		file := &responses.ResponseInputFileParam{Detail: responses.ResponseInputFileDetailHigh}
		if blob.URL != "" {
			file.FileURL = openaisdk.String(blob.URL)
			file.Filename = openaisdk.String(filenameFromURL(blob.URL))
		} else if len(blob.Data) > 0 {
			file.FileData = openaisdk.String(base64.StdEncoding.EncodeToString(blob.Data))
			file.Filename = openaisdk.String(defaultFilename(blob.MediaType))
		} else {
			return responses.ResponseInputContentUnionParam{}, false
		}

		return responses.ResponseInputContentUnionParam{OfInputFile: file}, true
	}

	return responses.ResponseInputContentUnionParam{}, false
}

func convertAssistantContentPart(part kit.ContentPart) (responses.ResponseInputItemUnionParam, bool) {
	switch part.Type {
	case kit.ContentTypeText, kit.ContentTypeSummary:
		return responses.ResponseInputItemUnionParam{
			OfMessage: &responses.EasyInputMessageParam{
				Role:    responses.EasyInputMessageRoleAssistant,
				Content: responses.EasyInputMessageContentUnionParam{OfString: openaisdk.String(part.TextValue())},
			},
		}, true
	case kit.ContentTypeThinking:
		thinking, ok := part.ThinkingValue()
		if !ok || thinking.ID == "" {
			return responses.ResponseInputItemUnionParam{}, false
		}

		return convertReasoningPart(part), true
	default:
		return responses.ResponseInputItemUnionParam{}, false
	}
}

func filenameFromURL(rawURL string) string {
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Path != "" {
		rawURL = parsed.Path
	}

	name := path.Base(rawURL)
	if name == "." || name == "/" || name == "" {
		return "document"
	}

	return name
}

func defaultFilename(mediaType string) string {
	switch mediaType {
	case "application/pdf":
		return "document.pdf"
	default:
		return "document"
	}
}

func convertTools(tools []kit.ToolDefinition) []responses.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	result := make([]responses.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		schema, err := json.Marshal(tool.Schema)
		if err != nil {
			continue
		}

		var parameters map[string]any
		if err := json.Unmarshal(schema, &parameters); err != nil {
			continue
		}

		result = append(result, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        tool.Name,
				Description: openaisdk.String(tool.Description),
				Parameters:  parameters,
			},
		})
	}

	return result
}

func convertResponse(resp *responses.Response) kit.ModelResponse {
	var (
		parts []kit.ContentPart
	)

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			msg := item.AsMessage()
			for _, part := range msg.Content {
				if text := part.AsOutputText(); text.Text != "" {
					parts = append(parts, kit.NewTextPart(text.Text))
				}
			}

		case "function_call":
			tc, err := convertFunctionCall(item)
			if err != nil {
				continue
			}

			parts = append(parts, kit.NewToolCallPart(tc))

		case "reasoning":
			reasoning := item.AsReasoning()

			text := reasoningText(reasoning)
			if text != "" || reasoning.EncryptedContent != "" {
				part := kit.NewThinkingPart(text, reasoning.EncryptedContent)
				part.Thinking.ID = reasoning.ID
				parts = append(parts, part)
			}
		}
	}

	return kit.ModelResponse{
		Message:      kit.NewAssistantMessage(parts...),
		FinishReason: convertStatus(resp),
		Usage: kit.Usage{
			InputTokens:     int32(resp.Usage.InputTokens),
			OutputTokens:    int32(resp.Usage.OutputTokens),
			CacheReadTokens: int32(resp.Usage.InputTokensDetails.CachedTokens),
			ReasoningTokens: int32(resp.Usage.OutputTokensDetails.ReasoningTokens),
		},
	}
}

func convertReasoningPart(part kit.ContentPart) responses.ResponseInputItemUnionParam {
	thinking, _ := part.ThinkingValue()

	reasoning := &responses.ResponseReasoningItemParam{
		ID:      thinking.ID,
		Summary: []responses.ResponseReasoningItemSummaryParam{{Text: thinking.Text}},
	}
	if thinking.Signature != "" {
		reasoning.EncryptedContent = param.NewOpt(thinking.Signature)
	}

	return responses.ResponseInputItemUnionParam{OfReasoning: reasoning}
}

func reasoningText(reasoning responses.ResponseReasoningItem) string {
	var out strings.Builder
	for _, summary := range reasoning.Summary {
		out.WriteString(summary.Text)
	}

	return out.String()
}

func convertStatus(resp *responses.Response) kit.FinishReason {
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

		return kit.FinishReasonMaxTokens
	default:
		return kit.FinishReasonUnknown
	}
}

func convertFunctionCall(item responses.ResponseOutputItemUnion) (kit.ToolCall, error) {
	var args map[string]any
	if s := item.Arguments.OfString; s != "" {
		if err := json.Unmarshal([]byte(s), &args); err != nil {
			return kit.ToolCall{}, fmt.Errorf("openai: unmarshal tool args: %w", err)
		}
	}

	return kit.ToolCall{
		ID:        item.CallID,
		Name:      item.Name,
		Arguments: args,
	}, nil
}
