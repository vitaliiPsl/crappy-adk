package google

import (
	"context"
	"iter"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// model implements [kit.Model] for a Gemini model.
type model struct {
	client *genai.Client
	config kit.ModelConfig
}

var _ kit.Model = (*model)(nil)

func (m *model) Config() kit.ModelConfig {
	return m.config
}

func (m *model) Generate(ctx context.Context, req kit.ModelRequest) (kit.ModelResponse, error) {
	contents := convertMessages(req.Messages)
	config := buildConfig(req)

	resp, err := m.client.Models.GenerateContent(ctx, m.config.ID, contents, config)
	if err != nil {
		return kit.ModelResponse{}, convertError(err)
	}

	return convertResponse(resp), nil
}

func (m *model) GenerateStream(ctx context.Context, req kit.ModelRequest) iter.Seq2[kit.ModelChunk, error] {
	return func(yield func(kit.ModelChunk, error) bool) {
		contents := convertMessages(req.Messages)
		config := buildConfig(req)

		for resp, err := range m.client.Models.GenerateContentStream(ctx, m.config.ID, contents, config) {
			if err != nil {
				yield(kit.ModelChunk{}, convertError(err))

				return
			}

			chunk := convertStreamChunk(resp)
			if !yield(chunk, nil) {
				return
			}
		}
	}
}

var thinkingBudgets = map[kit.ThinkingLevel]int32{
	kit.ThinkingLevelLow:    1024,
	kit.ThinkingLevelMedium: 8192,
	kit.ThinkingLevelHigh:   24576,
}

func buildConfig(req kit.ModelRequest) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(req.Instruction, genai.RoleUser),
		Tools:             convertTools(req.Tools),
		Temperature:       req.Config.Temperature,
		TopP:              req.Config.TopP,
	}

	if req.Config.MaxOutputTokens != nil {
		config.MaxOutputTokens = *req.Config.MaxOutputTokens
	}

	if budget, ok := thinkingBudgets[req.Config.Thinking]; ok {
		config.ThinkingConfig = &genai.ThinkingConfig{ThinkingBudget: &budget}
	}

	return config
}

func convertMessages(msgs []kit.Message) []*genai.Content {
	result := make([]*genai.Content, 0, len(msgs))
	for _, msg := range msgs {
		switch msg.Role {
		case kit.MessageRoleUser:
			result = append(result, convertUserMessage(msg))
		case kit.MessageRoleAssistant:
			result = append(result, convertAssistantMessage(msg))
		case kit.MessageRoleTool:
			result = append(result, convertToolMessage(msg))
		}
	}

	return result
}

func convertUserMessage(msg kit.Message) *genai.Content {
	return &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: msg.Content}},
	}
}

func convertAssistantMessage(msg kit.Message) *genai.Content {
	content := &genai.Content{
		Role:  genai.RoleModel,
		Parts: make([]*genai.Part, 0),
	}

	if msg.Content != "" {
		content.Parts = append(content.Parts, &genai.Part{Text: msg.Content})
	}

	for _, tc := range msg.ToolCalls {
		content.Parts = append(content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: tc.Name,
				Args: tc.Arguments,
			},
		})
	}

	return content
}

func convertToolMessage(msg kit.Message) *genai.Content {
	return &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				ID:       msg.ToolCallID,
				Name:     msg.ToolName,
				Response: map[string]any{"output": msg.Content},
			},
		}},
	}
}

func convertTools(tools []kit.ToolDefinition) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	decls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		decl := &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
		}
		if t.Schema != nil {
			decl.ParametersJsonSchema = t.Schema
		}
		decls = append(decls, decl)
	}

	return []*genai.Tool{{FunctionDeclarations: decls}}
}

func convertStreamChunk(resp *genai.GenerateContentResponse) kit.ModelChunk {
	if resp == nil || len(resp.Candidates) == 0 {
		return kit.ModelChunk{}
	}

	candidate := resp.Candidates[0]

	var chunk kit.ModelChunk
	if candidate.Content != nil {
		for _, p := range candidate.Content.Parts {
			if p.Thought {
				chunk.Thinking += p.Text
			} else if p.Text != "" {
				chunk.Text += p.Text
			}

			if p.FunctionCall != nil {
				id := p.FunctionCall.ID
				if id == "" {
					id = p.FunctionCall.Name
				}

				chunk.ToolCalls = append(chunk.ToolCalls, kit.ToolCall{
					ID:        id,
					Name:      p.FunctionCall.Name,
					Arguments: p.FunctionCall.Args,
				})
			}
		}
	}

	chunk.FinishReason = convertFinishReason(candidate.FinishReason, chunk.ToolCalls)

	if resp.UsageMetadata != nil {
		chunk.Usage = kit.Usage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	return chunk
}

func convertResponse(resp *genai.GenerateContentResponse) kit.ModelResponse {
	if resp == nil || len(resp.Candidates) == 0 {
		return kit.ModelResponse{}
	}

	candidate := resp.Candidates[0]

	var result kit.ModelResponse
	if candidate.Content != nil {
		for _, p := range candidate.Content.Parts {
			if p.Thought {
				result.Thinking += p.Text
			} else if p.Text != "" {
				result.Content += p.Text
			}

			if p.FunctionCall != nil {
				id := p.FunctionCall.ID
				if id == "" {
					id = p.FunctionCall.Name
				}

				result.ToolCalls = append(result.ToolCalls, kit.ToolCall{
					ID:        id,
					Name:      p.FunctionCall.Name,
					Arguments: p.FunctionCall.Args,
				})
			}
		}
	}

	result.FinishReason = convertFinishReason(candidate.FinishReason, result.ToolCalls)

	if resp.UsageMetadata != nil {
		result.Usage = kit.Usage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	return result
}

func convertFinishReason(r genai.FinishReason, toolCalls []kit.ToolCall) kit.FinishReason {
	if len(toolCalls) > 0 {
		return kit.FinishReasonToolCall
	}

	switch r {
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
