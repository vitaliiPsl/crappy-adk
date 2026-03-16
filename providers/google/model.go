package google

import (
	"context"
	"strings"

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

func (m *model) GenerateStream(ctx context.Context, req kit.ModelRequest) (*kit.ModelStream, error) {
	contents := convertMessages(req.Messages)
	config := buildConfig(req)

	iter := m.client.Models.GenerateContentStream(ctx, m.config.ID, contents, config)

	return kit.NewModelStream(func(yield func(kit.ModelChunk, error) bool) kit.ModelResponse {
		var (
			lastResp  *genai.GenerateContentResponse
			content   strings.Builder
			thinking  strings.Builder
			toolCalls []kit.ToolCall
		)

		for resp, err := range iter {
			if err != nil {
				yield(kit.ModelChunk{}, convertError(err))

				return kit.ModelResponse{}
			}

			lastResp = resp

			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, p := range resp.Candidates[0].Content.Parts {
					if p.Thought {
						thinking.WriteString(p.Text)

						if !yield(kit.NewThinkingChunk(p.Text), nil) {
							return kit.ModelResponse{}
						}
					} else if p.Text != "" {
						content.WriteString(p.Text)

						if !yield(kit.NewTextChunk(p.Text), nil) {
							return kit.ModelResponse{}
						}
					}

					if p.FunctionCall != nil {
						id := p.FunctionCall.ID
						if id == "" {
							id = p.FunctionCall.Name
						}

						tc := kit.ToolCall{
							ID:        id,
							Name:      p.FunctionCall.Name,
							Arguments: p.FunctionCall.Args,
						}

						if len(p.ThoughtSignature) > 0 {
							tc.Metadata = map[string]any{"thought_signature": p.ThoughtSignature}
						}

						toolCalls = append(toolCalls, tc)
						if !yield(kit.NewToolCallChunk(tc), nil) {
							return kit.ModelResponse{}
						}
					}
				}
			}
		}

		if lastResp == nil {
			return kit.ModelResponse{}
		}

		var finishReason genai.FinishReason
		if len(lastResp.Candidates) > 0 {
			finishReason = lastResp.Candidates[0].FinishReason
		}

		result := kit.ModelResponse{
			Message:      kit.NewAssistantMessage(content.String(), thinking.String(), toolCalls),
			FinishReason: convertFinishReason(finishReason, toolCalls),
		}

		if lastResp.UsageMetadata != nil {
			result.Usage = kit.Usage{
				InputTokens:     lastResp.UsageMetadata.PromptTokenCount,
				OutputTokens:    lastResp.UsageMetadata.CandidatesTokenCount,
				CacheReadTokens: lastResp.UsageMetadata.CachedContentTokenCount,
				ReasoningTokens: lastResp.UsageMetadata.ThoughtsTokenCount,
			}
		}

		return result
	}), nil
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
		config.ThinkingConfig = &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  &budget,
		}
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
	parts := make([]*genai.Part, 0, len(msg.Content))
	for _, p := range msg.Content {
		if part := convertContentPart(p); part != nil {
			parts = append(parts, part)
		}
	}

	return &genai.Content{Role: genai.RoleUser, Parts: parts}
}

func convertContentPart(p kit.ContentPart) *genai.Part {
	switch p.Type {
	case kit.ContentTypeText:
		return &genai.Part{Text: p.Text}
	case kit.ContentTypeImage, kit.ContentTypeDocument:
		if len(p.Data) > 0 {
			return &genai.Part{InlineData: &genai.Blob{Data: p.Data, MIMEType: p.MediaType}}
		}

		if p.URL != "" {
			return &genai.Part{FileData: &genai.FileData{FileURI: p.URL, MIMEType: p.MediaType}}
		}
	}

	return nil
}

func convertAssistantMessage(msg kit.Message) *genai.Content {
	content := &genai.Content{
		Role:  genai.RoleModel,
		Parts: make([]*genai.Part, 0),
	}

	if text := msg.Text(); text != "" {
		content.Parts = append(content.Parts, &genai.Part{Text: text})
	}

	for _, tc := range msg.ToolCalls {
		part := &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: tc.Name,
				Args: tc.Arguments,
			},
		}
		if tc.Metadata != nil {
			if sig, ok := tc.Metadata["thought_signature"].([]byte); ok {
				part.ThoughtSignature = sig
			}
		}

		content.Parts = append(content.Parts, part)
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
				Response: map[string]any{"output": msg.Text()},
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

func convertResponse(resp *genai.GenerateContentResponse) kit.ModelResponse {
	if resp == nil || len(resp.Candidates) == 0 {
		return kit.ModelResponse{}
	}

	candidate := resp.Candidates[0]

	var (
		content   strings.Builder
		thinking  strings.Builder
		toolCalls []kit.ToolCall
	)

	if candidate.Content != nil {
		for _, p := range candidate.Content.Parts {
			if p.Thought {
				thinking.WriteString(p.Text)
			} else if p.Text != "" {
				content.WriteString(p.Text)
			}

			if p.FunctionCall != nil {
				id := p.FunctionCall.ID
				if id == "" {
					id = p.FunctionCall.Name
				}

				tc := kit.ToolCall{
					ID:        id,
					Name:      p.FunctionCall.Name,
					Arguments: p.FunctionCall.Args,
				}
				if len(p.ThoughtSignature) > 0 {
					tc.Metadata = map[string]any{"thought_signature": p.ThoughtSignature}
				}

				toolCalls = append(toolCalls, tc)
			}
		}
	}

	result := kit.ModelResponse{
		Message:      kit.NewAssistantMessage(content.String(), thinking.String(), toolCalls),
		FinishReason: convertFinishReason(candidate.FinishReason, toolCalls),
	}

	if resp.UsageMetadata != nil {
		result.Usage = kit.Usage{
			InputTokens:     resp.UsageMetadata.PromptTokenCount,
			OutputTokens:    resp.UsageMetadata.CandidatesTokenCount,
			CacheReadTokens: resp.UsageMetadata.CachedContentTokenCount,
			ReasoningTokens: resp.UsageMetadata.ThoughtsTokenCount,
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
