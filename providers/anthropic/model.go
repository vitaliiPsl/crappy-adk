package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

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
	params := buildParams(req, m.config)

	resp, err := m.client.Messages.New(ctx, params)
	if err != nil {
		return kit.ModelResponse{}, convertError(err)
	}

	return convertResponse(resp), nil
}

func (m *model) GenerateStream(ctx context.Context, req kit.ModelRequest) iter.Seq2[kit.ModelChunk, error] {
	return func(yield func(kit.ModelChunk, error) bool) {
		params := buildParams(req, m.config)

		stream := m.client.Messages.NewStreaming(ctx, params)
		defer func() { _ = stream.Close() }()

		var message anthropic.Message

		for stream.Next() {
			event := stream.Current()

			err := message.Accumulate(event)
			if err != nil {
				yield(kit.ModelChunk{}, fmt.Errorf("anthropic: accumulate message: %w", err))

				return
			}

			switch e := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch d := e.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					if !yield(kit.ModelChunk{Text: d.Text}, nil) {
						return
					}

				case anthropic.ThinkingDelta:
					if !yield(kit.ModelChunk{Thinking: d.Thinking}, nil) {
						return
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			yield(kit.ModelChunk{}, convertError(err))

			return
		}

		// Extract tool calls and emit the final chunk from the accumulated message.
		var toolCalls []kit.ToolCall
		for _, cb := range message.Content {
			tu, ok := cb.AsAny().(anthropic.ToolUseBlock)
			if !ok {
				continue
			}
			tc, err := parseToolCall(tu.ID, tu.Name, string(tu.Input))
			if err != nil {
				yield(kit.ModelChunk{}, err)

				return
			}
			toolCalls = append(toolCalls, tc)
		}

		yield(kit.ModelChunk{
			ToolCalls:    toolCalls,
			FinishReason: convertStopReason(message.StopReason),
			Usage: kit.Usage{
				InputTokens:  int32(message.Usage.InputTokens),
				OutputTokens: int32(message.Usage.OutputTokens),
			},
		}, nil)
	}
}

func buildParams(req kit.ModelRequest, cfg kit.ModelConfig) anthropic.MessageNewParams {
	gc := req.Config

	params := anthropic.MessageNewParams{
		Model:    anthropic.Model(cfg.ID),
		Messages: convertMessages(req.Messages),
		Tools:    convertTools(req.Tools),
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

	maxTokens := cfg.MaxOutputTokens
	if gc.MaxOutputTokens != nil {
		maxTokens = int64(*gc.MaxOutputTokens)
	}
	params.MaxTokens = maxTokens

	if gc.Thinking != kit.ThinkingDisabled {
		budget := thinkingBudget(gc.Thinking)
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(budget)
	}

	return params
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
		switch msg.Role {
		case kit.MessageRoleUser:
			result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case kit.MessageRoleAssistant:
			result = append(result, convertAssistantMessage(msg))
		case kit.MessageRoleTool:
			// Consecutive turns are combined by the API into a single turn
			result = append(result, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
			))
		}
	}

	return result
}

func convertAssistantMessage(msg kit.Message) anthropic.MessageParam {
	var blocks []anthropic.ContentBlockParamUnion

	if msg.Thinking != "" {
		blocks = append(blocks, anthropic.NewThinkingBlock("", msg.Thinking))
	}

	if msg.Content != "" {
		blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
	}

	for _, tc := range msg.ToolCalls {
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
	}

	return anthropic.NewAssistantMessage(blocks...)
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
	result := kit.ModelResponse{
		FinishReason: convertStopReason(resp.StopReason),
		Usage: kit.Usage{
			InputTokens:  int32(resp.Usage.InputTokens),
			OutputTokens: int32(resp.Usage.OutputTokens),
		},
	}

	for _, cb := range resp.Content {
		switch v := cb.AsAny().(type) {
		case anthropic.TextBlock:
			result.Content += v.Text
		case anthropic.ThinkingBlock:
			result.Thinking += v.Thinking
		case anthropic.ToolUseBlock:
			tc, err := parseToolCall(v.ID, v.Name, string(v.Input))
			if err != nil {
				continue
			}
			result.ToolCalls = append(result.ToolCalls, tc)
		}
	}

	return result
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
