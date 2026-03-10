package google

import (
	"context"

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
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(req.Instructions, genai.RoleUser),
		Tools:             convertTools(req.Tools),
	}

	resp, err := m.client.Models.GenerateContent(ctx, m.config.ID, contents, config)
	if err != nil {
		return kit.ModelResponse{}, convertError(err)
	}

	return convertResponse(resp), nil
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

func convertResponse(resp *genai.GenerateContentResponse) kit.ModelResponse {
	if resp == nil || len(resp.Candidates) == 0 {
		return kit.ModelResponse{}
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return kit.ModelResponse{}
	}

	var result kit.ModelResponse
	for _, p := range candidate.Content.Parts {
		if p.Text != "" {
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

	return result
}
