package google

import (
	"context"
	"encoding/base64"
	"strings"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// encodeSignature base64-encodes a Gemini thought signature so the raw binary
// bytes survive JSON round-trips through [kit.ContentPart.Signature].
func encodeSignature(sig []byte) string {
	if len(sig) == 0 {
		return ""
	}

	return base64.StdEncoding.EncodeToString(sig)
}

// decodeSignature reverses [encodeSignature]. Returns nil for the empty string
// or when the value isn't valid base64 (shouldn't happen for provider-issued
// signatures but avoids sending garbage back).
func decodeSignature(sig string) []byte {
	if sig == "" {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return nil
	}

	return decoded
}

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

func (m *model) GenerateStream(ctx context.Context, req kit.ModelRequest) (*kit.Stream[kit.ModelEvent, kit.ModelResponse], error) {
	contents := convertMessages(req.Messages)
	config := buildConfig(req)

	iter := m.client.Models.GenerateContentStream(ctx, m.config.ID, contents, config)

	return kit.NewStream(func(yield func(kit.ModelEvent, error) bool) kit.ModelResponse {
		var (
			lastResp           *genai.GenerateContentResponse
			content            []kit.ContentPart
			thinkingStarted    bool
			contentPartStarted bool
		)

		for resp, err := range iter {
			if err != nil {
				yield(kit.ModelEvent{}, convertError(err))

				return kit.ModelResponse{}
			}

			lastResp = resp

			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, p := range resp.Candidates[0].Content.Parts {
					if p.Thought {
						content = appendThinkingDelta(content, p.Text, p.ThoughtSignature)

						if !thinkingStarted {
							thinkingStarted = true

							if !yield(kit.NewModelContentPartStartedEvent(kit.ContentTypeThinking), nil) {
								return kit.ModelResponse{}
							}
						}

						if !yield(kit.NewModelContentPartDeltaEvent(kit.ContentTypeThinking, p.Text), nil) {
							return kit.ModelResponse{}
						}
					} else if p.Text != "" {
						content = appendTextDelta(content, p.Text)

						if !contentPartStarted {
							contentPartStarted = true

							if !yield(kit.NewModelContentPartStartedEvent(kit.ContentTypeText), nil) {
								return kit.ModelResponse{}
							}
						}

						if !yield(kit.NewModelContentPartDeltaEvent(kit.ContentTypeText, p.Text), nil) {
							return kit.ModelResponse{}
						}
					} else if part, ok := convertResponsePart(p); ok {
						content = append(content, part)
					}

					if p.FunctionCall != nil {
						toolPart := toolCallPart(p)
						content = append(content, toolPart)

						if !yield(kit.NewModelContentPartStartedEvent(kit.ContentTypeToolCall), nil) {
							return kit.ModelResponse{}
						}

						if !yield(kit.NewModelContentPartDoneEvent(toolPart), nil) {
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
			Message: kit.Message{
				Role:    kit.MessageRoleAssistant,
				Content: content,
			},
			FinishReason: convertFinishReason(finishReason, content),
		}

		if lastResp.UsageMetadata != nil {
			result.Usage = kit.Usage{
				InputTokens:     lastResp.UsageMetadata.PromptTokenCount,
				OutputTokens:    lastResp.UsageMetadata.CandidatesTokenCount,
				CacheReadTokens: lastResp.UsageMetadata.CachedContentTokenCount,
				ReasoningTokens: lastResp.UsageMetadata.ThoughtsTokenCount,
			}
		}

		for _, part := range result.Message.Content {
			if part.Type == kit.ContentTypeRedactedThinking || part.Type == kit.ContentTypeToolCall {
				continue
			}

			if !yield(kit.NewModelContentPartDoneEvent(part), nil) {
				return result
			}
		}

		return result
	}), nil
}

func toolCallPart(p *genai.Part) kit.ContentPart {
	id := p.FunctionCall.ID
	if id == "" {
		id = p.FunctionCall.Name
	}

	part := kit.ContentPart{
		Type:      kit.ContentTypeToolCall,
		ID:        id,
		Name:      p.FunctionCall.Name,
		Arguments: p.FunctionCall.Args,
	}
	if len(p.ThoughtSignature) > 0 {
		part.Signature = encodeSignature(p.ThoughtSignature)
	}

	return part
}

func appendThinkingDelta(parts []kit.ContentPart, text string, signature []byte) []kit.ContentPart {
	if len(parts) > 0 && parts[len(parts)-1].Type == kit.ContentTypeThinking {
		parts[len(parts)-1].Text += text
		if len(signature) > 0 {
			parts[len(parts)-1].Signature = encodeSignature(signature)
		}

		return parts
	}

	return append(parts, kit.NewThinkingPart(text, encodeSignature(signature)))
}

func appendTextDelta(parts []kit.ContentPart, text string) []kit.ContentPart {
	if len(parts) > 0 && parts[len(parts)-1].Type == kit.ContentTypeText {
		parts[len(parts)-1].Text += text

		return parts
	}

	return append(parts, kit.NewTextPart(text))
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
		if content := convertMessage(msg); content != nil {
			result = append(result, content)
		}
	}

	return result
}

func convertMessage(msg kit.Message) *genai.Content {
	switch msg.Role {
	case kit.MessageRoleUser:
		return convertUserMessage(msg)
	case kit.MessageRoleAssistant:
		return convertAssistantMessage(msg)
	case kit.MessageRoleTool:
		return convertToolMessage(msg)
	default:
		return nil
	}
}

func convertUserMessage(msg kit.Message) *genai.Content {
	parts := make([]*genai.Part, 0, len(msg.Content))
	for _, p := range msg.Content {
		if part := convertUserContentPart(p); part != nil {
			parts = append(parts, part)
		}
	}

	return &genai.Content{Role: genai.RoleUser, Parts: parts}
}

func convertUserContentPart(p kit.ContentPart) *genai.Part {
	switch p.Type {
	case kit.ContentTypeText:
		return &genai.Part{Text: p.Text}
	case kit.ContentTypeImage, kit.ContentTypeDocument:
		if len(p.Data) > 0 {
			return &genai.Part{
				InlineData: &genai.Blob{Data: p.Data, MIMEType: p.MediaType},
			}
		}

		if p.URL != "" {
			return &genai.Part{
				FileData: &genai.FileData{FileURI: p.URL, MIMEType: p.MediaType},
			}
		}
	}

	return nil
}

func convertAssistantMessage(msg kit.Message) *genai.Content {
	content := &genai.Content{
		Role:  genai.RoleModel,
		Parts: make([]*genai.Part, 0, len(msg.Content)),
	}

	for _, part := range msg.Content {
		if part.Type == kit.ContentTypeToolCall {
			content.Parts = append(content.Parts, toolCallPartToGenai(part))

			continue
		}

		if converted := convertAssistantContentPart(part); converted != nil {
			content.Parts = append(content.Parts, converted)
		}
	}

	return content
}

func toolCallPartToGenai(part kit.ContentPart) *genai.Part {
	p := &genai.Part{
		FunctionCall: &genai.FunctionCall{
			ID:   part.ID,
			Name: part.Name,
			Args: part.Arguments,
		},
	}
	if sig := decodeSignature(part.Signature); len(sig) > 0 {
		p.ThoughtSignature = sig
	}

	return p
}

func convertAssistantContentPart(part kit.ContentPart) *genai.Part {
	switch part.Type {
	case kit.ContentTypeThinking:
		p := &genai.Part{
			Text:    part.Text,
			Thought: true,
		}
		if sig := decodeSignature(part.Signature); len(sig) > 0 {
			p.ThoughtSignature = sig
		}

		return p
	case kit.ContentTypeRedactedThinking:
		if len(part.Data) == 0 {
			return nil
		}

		return &genai.Part{
			Thought:          true,
			ThoughtSignature: part.Data,
		}
	default:
		return convertUserContentPart(part)
	}
}

func convertToolMessage(msg kit.Message) *genai.Content {
	callID := ""
	name := ""

	output := msg.Text()
	if part, ok := msg.ToolResult(); ok {
		callID = part.ID
		name = part.Name
		output = part.Text
	}

	return &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				ID:       callID,
				Name:     name,
				Response: map[string]any{"output": output},
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

	var content []kit.ContentPart

	if candidate.Content != nil {
		for _, p := range candidate.Content.Parts {
			if p.Thought {
				content = append(content, kit.NewThinkingPart(p.Text, encodeSignature(p.ThoughtSignature)))
			} else if part, ok := convertResponsePart(p); ok {
				content = append(content, part)
			}

			if p.FunctionCall != nil {
				content = append(content, toolCallPart(p))
			}
		}
	}

	result := kit.ModelResponse{
		Message: kit.Message{
			Role:    kit.MessageRoleAssistant,
			Content: content,
		},
		FinishReason: convertFinishReason(candidate.FinishReason, content),
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

func convertResponsePart(p *genai.Part) (kit.ContentPart, bool) {
	if p == nil {
		return kit.ContentPart{}, false
	}

	var part kit.ContentPart
	switch {
	case p.Text != "":
		part = kit.NewTextPart(p.Text)
	case p.InlineData != nil:
		if strings.HasPrefix(p.InlineData.MIMEType, "image/") {
			part = kit.NewImageDataPart(p.InlineData.Data, p.InlineData.MIMEType)
		} else {
			part = kit.NewDocumentDataPart(p.InlineData.Data, p.InlineData.MIMEType)
		}
	case p.FileData != nil:
		if strings.HasPrefix(p.FileData.MIMEType, "image/") {
			part = kit.ContentPart{Type: kit.ContentTypeImage, URL: p.FileData.FileURI, MediaType: p.FileData.MIMEType}
		} else {
			part = kit.ContentPart{Type: kit.ContentTypeDocument, URL: p.FileData.FileURI, MediaType: p.FileData.MIMEType}
		}
	default:
		return kit.ContentPart{}, false
	}

	return part, true
}

func convertFinishReason(r genai.FinishReason, content []kit.ContentPart) kit.FinishReason {
	for _, p := range content {
		if p.Type == kit.ContentTypeToolCall {
			return kit.FinishReasonToolCall
		}
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
