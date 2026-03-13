package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/vitaliiPsl/crappy-adk/kit"
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
	params := buildParams(req, m.config.ID)

	resp, err := m.client.Responses.New(ctx, params)
	if err != nil {
		return kit.ModelResponse{}, convertError(err)
	}

	return convertResponse(resp), nil
}

func (m *model) GenerateStream(ctx context.Context, req kit.ModelRequest) iter.Seq2[kit.ModelChunk, error] {
	return func(yield func(kit.ModelChunk, error) bool) {
		params := buildParams(req, m.config.ID)

		stream := m.client.Responses.NewStreaming(ctx, params)
		defer func() { _ = stream.Close() }()

		for stream.Next() {
			event := stream.Current()

			switch e := event.AsAny().(type) {
			case responses.ResponseTextDeltaEvent:
				if !yield(kit.ModelChunk{Text: e.Delta}, nil) {
					return
				}

			case responses.ResponseReasoningTextDeltaEvent:
				if !yield(kit.ModelChunk{Thinking: e.Delta}, nil) {
					return
				}

			case responses.ResponseOutputItemDoneEvent:
				if e.Item.Type != "function_call" {
					continue
				}

				tc, err := convertFunctionCall(e.Item)
				if err != nil {
					yield(kit.ModelChunk{}, err)

					return
				}

				if !yield(kit.ModelChunk{ToolCalls: []kit.ToolCall{tc}}, nil) {
					return
				}

			case responses.ResponseCompletedEvent:
				resp := e.Response
				if !yield(kit.ModelChunk{
					FinishReason: convertStatus(&resp),
					Usage: kit.Usage{
						InputTokens:  int32(resp.Usage.InputTokens),
						OutputTokens: int32(resp.Usage.OutputTokens),
					},
				}, nil) {
					return
				}
			}
		}

		if err := stream.Err(); err != nil {
			yield(kit.ModelChunk{}, convertError(err))
		}
	}
}

func buildParams(req kit.ModelRequest, modelID string) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model: modelID,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: convertMessages(req.Messages),
		},
		Tools: convertTools(req.Tools),
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
			Effort: reasoningEffort(gc.Thinking),
		}
	}

	return params
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
		result = append(result, convertInputItems(msg)...)
	}

	return result
}

func convertInputItems(msg kit.Message) []responses.ResponseInputItemUnionParam {
	switch msg.Role {
	case kit.MessageRoleUser:
		return []responses.ResponseInputItemUnionParam{{
			OfMessage: &responses.EasyInputMessageParam{
				Role:    responses.EasyInputMessageRoleUser,
				Content: responses.EasyInputMessageContentUnionParam{OfString: openaisdk.String(msg.Content)},
			},
		}}

	case kit.MessageRoleAssistant:
		var items []responses.ResponseInputItemUnionParam

		if msg.Content != "" {
			items = append(items, responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Role:    responses.EasyInputMessageRoleAssistant,
					Content: responses.EasyInputMessageContentUnionParam{OfString: openaisdk.String(msg.Content)},
				},
			})
		}

		for _, tc := range msg.ToolCalls {
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

	case kit.MessageRoleTool:
		return []responses.ResponseInputItemUnionParam{{
			OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
				CallID: msg.ToolCallID,
				Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
					OfString: openaisdk.String(msg.Content),
				},
			},
		}}

	default:
		return []responses.ResponseInputItemUnionParam{{
			OfMessage: &responses.EasyInputMessageParam{
				Role:    responses.EasyInputMessageRoleUser,
				Content: responses.EasyInputMessageContentUnionParam{OfString: openaisdk.String(msg.Content)},
			},
		}}
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
	var content, thinking string
	var toolCalls []kit.ToolCall

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			msg := item.AsMessage()
			for _, part := range msg.Content {
				if text := part.AsOutputText(); text.Text != "" {
					content += text.Text
				}
			}

		case "function_call":
			tc, err := convertFunctionCall(item)
			if err != nil {
				continue
			}
			toolCalls = append(toolCalls, tc)

		case "reasoning":
			reasoning := item.AsReasoning()
			for _, summary := range reasoning.Summary {
				thinking += summary.Text
			}
		}
	}

	return kit.ModelResponse{
		Message:      kit.NewAssistantMessage(content, thinking, toolCalls),
		FinishReason: convertStatus(resp),
		Usage: kit.Usage{
			InputTokens:  int32(resp.Usage.InputTokens),
			OutputTokens: int32(resp.Usage.OutputTokens),
		},
	}
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
