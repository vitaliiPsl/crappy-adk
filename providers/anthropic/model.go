package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/vitaliiPsl/crappy-adk/kit"
	xstream "github.com/vitaliiPsl/crappy-adk/x/stream"
	"github.com/vitaliiPsl/crappy-adk/x/structuredoutput"
)

const defaultMaxOutputTokens int64 = 8192

// model implements [kit.Model] for an Anthropic model via the Messages API.
type model struct {
	client *anthropic.Client
	config kit.ModelConfig
}

var _ kit.Model = (*model)(nil)

func (m *model) Config() kit.ModelConfig {
	return m.config
}

func (m *model) Generate(ctx context.Context, req kit.ModelRequest) (kit.ModelResponse, error) {
	params, err := buildParams(req, m.config)
	if err != nil {
		return kit.ModelResponse{}, err
	}

	resp, err := m.client.Messages.New(ctx, params)
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

func (m *model) GenerateStream(ctx context.Context, req kit.ModelRequest) (*xstream.Stream[kit.ModelEvent, kit.ModelResponse], error) {
	params, err := buildParams(req, m.config)
	if err != nil {
		return nil, err
	}

	stream := m.client.Messages.NewStreaming(ctx, params)

	return xstream.New(func(yield func(kit.ModelEvent, error) bool) kit.ModelResponse {
		defer func() { _ = stream.Close() }()

		var (
			message        anthropic.Message
			currentBlock   anthropic.ContentBlockStartEvent
			currentPart    *kit.ContentPart
			builder        strings.Builder
			toolStartInput string
		)

		for stream.Next() {
			event := stream.Current()

			if err := message.Accumulate(event); err != nil {
				yield(kit.ModelEvent{}, fmt.Errorf("anthropic: accumulate message: %w", err))

				return kit.ModelResponse{}
			}

			switch e := event.AsAny().(type) {
			case anthropic.ContentBlockStartEvent:
				currentBlock = e
				currentPart = nil

				builder.Reset()

				toolStartInput = ""

				switch block := e.ContentBlock.AsAny().(type) {
				case anthropic.TextBlock:
					part := kit.NewTextPart("")
					currentPart = &part

					if !yield(kit.NewModelContentPartStartedEvent(kit.ContentTypeText), nil) {
						return kit.ModelResponse{}
					}
				case anthropic.ThinkingBlock:
					part := kit.NewThinkingPart("", block.Signature)
					currentPart = &part

					if !yield(kit.NewModelContentPartStartedEvent(kit.ContentTypeThinking), nil) {
						return kit.ModelResponse{}
					}
				case anthropic.ToolUseBlock:
					toolStartInput = strings.TrimSpace(string(block.Input))
					part := kit.NewToolCallPart(kit.ToolCall{
						ID:   block.ID,
						Name: block.Name,
					})
					currentPart = &part

					if !yield(kit.NewModelContentPartStartedEvent(kit.ContentTypeToolCall), nil) {
						return kit.ModelResponse{}
					}
				}

			case anthropic.ContentBlockDeltaEvent:
				if currentPart == nil || e.Index != currentBlock.Index {
					continue
				}

				switch d := e.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					builder.WriteString(d.Text)

					if !yield(kit.NewModelContentPartDeltaEvent(kit.ContentTypeText, d.Text), nil) {
						return kit.ModelResponse{}
					}

				case anthropic.ThinkingDelta:
					builder.WriteString(d.Thinking)

					if !yield(kit.NewModelContentPartDeltaEvent(kit.ContentTypeThinking, d.Thinking), nil) {
						return kit.ModelResponse{}
					}

				case anthropic.InputJSONDelta:
					builder.WriteString(d.PartialJSON)
				}

			case anthropic.ContentBlockStopEvent:
				if currentPart == nil || e.Index != currentBlock.Index {
					continue
				}

				switch currentPart.Type {
				case kit.ContentTypeText, kit.ContentTypeThinking:
					currentPart.Text = builder.String()

				case kit.ContentTypeToolCall:
					args, err := parseToolUseInput(builder.String(), toolStartInput)
					if err != nil {
						yield(kit.ModelEvent{}, err)

						return kit.ModelResponse{}
					}

					currentPart.Arguments = args
				}

				if !yield(kit.NewModelContentPartDoneEvent(*currentPart), nil) {
					return kit.ModelResponse{}
				}

				currentPart = nil
			}
		}

		if err := stream.Err(); err != nil {
			yield(kit.ModelEvent{}, convertError(err))

			return kit.ModelResponse{}
		}

		resp := convertResponse(&message)

		resp.StructuredOutput, err = structuredoutput.Validate(resp.Message.Text(), req.ResponseSchema)
		if err != nil {
			yield(kit.ModelEvent{}, &kit.LLMError{
				Kind:      kit.ErrStructuredOutput,
				Message:   err.Error(),
				Retryable: false,
				Provider:  ProviderID,
				Cause:     err,
			})

			return kit.ModelResponse{}
		}

		return resp
	}), nil
}

func parseToolUseInput(raw string, fallback string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = strings.TrimSpace(fallback)
	}

	if raw == "" {
		return nil, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("anthropic: unmarshal tool args: %w", err)
	}

	return args, nil
}

func buildParams(req kit.ModelRequest, cfg kit.ModelConfig) (anthropic.MessageNewParams, error) {
	gc := req.Config

	params := anthropic.MessageNewParams{
		Model:    anthropic.Model(cfg.ID),
		Messages: convertMessages(req.Messages),
		Tools:    convertTools(req.Tools),
	}

	if req.ResponseSchema != nil {
		schema, err := structuredoutput.SchemaMap(req.ResponseSchema)
		if err != nil {
			return anthropic.MessageNewParams{}, fmt.Errorf("anthropic: schema map: %w", err)
		}

		params.OutputConfig = anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		}
	}

	if req.Instruction != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.Instruction}}
	}

	if gc.Temperature != nil {
		params.Temperature = param.NewOpt(float64(*gc.Temperature))
	}

	if gc.TopP != nil {
		params.TopP = param.NewOpt(float64(*gc.TopP))
	}

	// Anthropic requires max_tokens >= 1
	maxTokens := defaultMaxTokens(cfg.OutputLimit)
	if gc.MaxOutputTokens != nil {
		maxTokens = int64(*gc.MaxOutputTokens)
	}

	params.MaxTokens = maxTokens

	if gc.Thinking != kit.ThinkingDisabled {
		budget := thinkingBudget(gc.Thinking)
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(budget)
	}

	return params, nil
}

func defaultMaxTokens(limit int) int64 {
	if limit > 0 {
		return int64(limit)
	}

	return defaultMaxOutputTokens
}

var thinkingBudgets = map[kit.ThinkingLevel]int64{
	kit.ThinkingLevelLow:    1024,
	kit.ThinkingLevelMedium: 8192,
	kit.ThinkingLevelHigh:   16384,
}

func thinkingBudget(level kit.ThinkingLevel) int64 {
	if b, ok := thinkingBudgets[level]; ok {
		return b
	}

	return 8192
}

func convertMessages(msgs []kit.Message) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, 0, len(msgs))

	for _, msg := range msgs {
		if converted := convertMessage(msg); len(converted) > 0 {
			result = append(result, converted...)
		}
	}

	return result
}

func convertMessage(msg kit.Message) []anthropic.MessageParam {
	switch msg.Role {
	case kit.MessageRoleUser:
		return []anthropic.MessageParam{convertUserMessage(msg)}
	case kit.MessageRoleAssistant:
		return []anthropic.MessageParam{convertAssistantMessage(msg)}
	case kit.MessageRoleTool:
		return []anthropic.MessageParam{convertToolMessage(msg)}
	default:
		return nil
	}
}

func convertUserMessage(msg kit.Message) anthropic.MessageParam {
	return anthropic.NewUserMessage(convertUserContentParts(msg.Content)...)
}

func convertAssistantMessage(msg kit.Message) anthropic.MessageParam {
	var blocks []anthropic.ContentBlockParamUnion

	seenToolCalls := make(map[string]struct{})

	for _, p := range msg.Content {
		switch p.Type {
		case kit.ContentTypeThinking:
			blocks = append(blocks, anthropic.NewThinkingBlock(p.Signature, p.Text))
		case kit.ContentTypeRedactedThinking:
			blocks = append(blocks, anthropic.NewRedactedThinkingBlock(string(p.Data)))
		case kit.ContentTypeText:
			if p.Text != "" {
				blocks = append(blocks, anthropic.NewTextBlock(p.Text))
			}
		case kit.ContentTypeToolCall:
			blocks = append(blocks, anthropic.NewToolUseBlock(p.ID, p.Arguments, p.Name))
			seenToolCalls[p.ID] = struct{}{}
		}
	}

	for _, tc := range msg.ToolCalls() {
		if _, ok := seenToolCalls[tc.ID]; ok {
			continue
		}

		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
	}

	return anthropic.NewAssistantMessage(blocks...)
}

func convertToolMessage(msg kit.Message) anthropic.MessageParam {
	callID := ""

	output := msg.Text()
	if part, ok := msg.ToolResult(); ok {
		callID = part.ID
		output = part.Text
	}

	// Consecutive turns are combined by the API into a single turn.
	return anthropic.NewUserMessage(
		anthropic.NewToolResultBlock(callID, output, false),
	)
}

func convertUserContentParts(parts []kit.ContentPart) []anthropic.ContentBlockParamUnion {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(parts))
	for _, p := range parts {
		if block, ok := convertUserContentPart(p); ok {
			blocks = append(blocks, block)
		}
	}

	return blocks
}

func convertUserContentPart(p kit.ContentPart) (anthropic.ContentBlockParamUnion, bool) {
	switch p.Type {
	case kit.ContentTypeText:
		return anthropic.NewTextBlock(p.Text), true
	case kit.ContentTypeImage:
		if len(p.Data) > 0 {
			return anthropic.NewImageBlockBase64(p.MediaType, base64.StdEncoding.EncodeToString(p.Data)), true
		}

		if p.URL != "" {
			return anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: p.URL}), true
		}
	case kit.ContentTypeDocument:
		if len(p.Data) > 0 {
			return anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: base64.StdEncoding.EncodeToString(p.Data)}), true
		}

		if p.URL != "" {
			return anthropic.NewDocumentBlock(anthropic.URLPDFSourceParam{URL: p.URL}), true
		}
	}

	return anthropic.ContentBlockParamUnion{}, false
}

func convertTools(tools []kit.ToolDefinition) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		schema, err := json.Marshal(tool.Schema)
		if err != nil {
			continue
		}

		var schemaMap map[string]any
		if err := json.Unmarshal(schema, &schemaMap); err != nil {
			continue
		}

		inputSchema := anthropic.ToolInputSchemaParam{
			Properties: schemaMap["properties"],
		}
		if req, ok := schemaMap["required"].([]any); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					inputSchema.Required = append(inputSchema.Required, s)
				}
			}
		}

		result = append(result, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: param.NewOpt(tool.Description),
				InputSchema: inputSchema,
			},
		})
	}

	return result
}

func convertResponse(resp *anthropic.Message) kit.ModelResponse {
	var (
		parts []kit.ContentPart
	)

	for _, cb := range resp.Content {
		switch v := cb.AsAny().(type) {
		case anthropic.TextBlock:
			parts = append(parts, kit.NewTextPart(v.Text))
		case anthropic.ThinkingBlock:
			parts = append(parts, kit.NewThinkingPart(v.Thinking, v.Signature))
		case anthropic.RedactedThinkingBlock:
			parts = append(parts, kit.NewRedactedThinkingPart([]byte(v.Data)))
		case anthropic.ToolUseBlock:
			tc, err := parseToolCall(v.ID, v.Name, string(v.Input))
			if err != nil {
				continue
			}

			parts = append(parts, kit.NewToolCallPart(tc))
		}
	}

	return kit.ModelResponse{
		Message: kit.Message{
			Role:    kit.MessageRoleAssistant,
			Content: parts,
		},
		FinishReason: convertStopReason(resp.StopReason),
		Usage: kit.Usage{
			InputTokens:      int32(resp.Usage.InputTokens),
			OutputTokens:     int32(resp.Usage.OutputTokens),
			CacheReadTokens:  int32(resp.Usage.CacheReadInputTokens),
			CacheWriteTokens: int32(resp.Usage.CacheCreationInputTokens),
		},
	}
}

func convertStopReason(r anthropic.StopReason) kit.FinishReason {
	switch r {
	case anthropic.StopReasonEndTurn:
		return kit.FinishReasonStop
	case anthropic.StopReasonMaxTokens:
		return kit.FinishReasonMaxTokens
	case anthropic.StopReasonToolUse:
		return kit.FinishReasonToolCall
	case anthropic.StopReasonStopSequence:
		return kit.FinishReasonStop
	default:
		return kit.FinishReasonUnknown
	}
}

func parseToolCall(id, name, rawJSON string) (kit.ToolCall, error) {
	var args map[string]any
	if rawJSON != "" {
		if err := json.Unmarshal([]byte(rawJSON), &args); err != nil {
			return kit.ToolCall{}, fmt.Errorf("anthropic: unmarshal tool args: %w", err)
		}
	}

	return kit.ToolCall{ID: id, Name: name, Arguments: args}, nil
}
