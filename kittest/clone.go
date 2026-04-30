package kittest

import (
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func cloneModelRequest(req kit.ModelRequest) kit.ModelRequest {
	cloned := kit.ModelRequest{
		Instruction: req.Instruction,
		Config:      cloneGenerationConfig(req.Config),
	}

	if req.ResponseSchema != nil {
		cloned.ResponseSchema = cloneSchema(req.ResponseSchema)
	}

	if len(req.Messages) > 0 {
		cloned.Messages = make([]kit.Message, len(req.Messages))
		for i, msg := range req.Messages {
			cloned.Messages[i] = cloneMessage(msg)
		}
	}

	if len(req.Tools) > 0 {
		cloned.Tools = make([]kit.ToolDefinition, len(req.Tools))
		copy(cloned.Tools, req.Tools)
	}

	return cloned
}

func cloneMessage(msg kit.Message) kit.Message {
	cloned := kit.Message{
		Role: msg.Role,
	}

	if len(msg.Content) > 0 {
		cloned.Content = make([]kit.ContentPart, len(msg.Content))
		for i, part := range msg.Content {
			cloned.Content[i] = cloneContentPart(part)
		}
	}

	return cloned
}

func cloneContentPart(part kit.ContentPart) kit.ContentPart {
	cloned := part

	if part.Image != nil {
		img := *part.Image
		img.Data = append([]byte(nil), part.Image.Data...)
		cloned.Image = &img
	}

	if part.Document != nil {
		doc := *part.Document
		doc.Data = append([]byte(nil), part.Document.Data...)
		cloned.Document = &doc
	}

	if part.RedactedThinking != nil {
		redacted := *part.RedactedThinking
		redacted.Data = append([]byte(nil), part.RedactedThinking.Data...)
		cloned.RedactedThinking = &redacted
	}

	if part.Text != nil {
		text := *part.Text
		cloned.Text = &text
	}

	if part.Summary != nil {
		summary := *part.Summary
		cloned.Summary = &summary
	}

	if part.Thinking != nil {
		thinking := *part.Thinking
		cloned.Thinking = &thinking
	}

	if part.ToolCall != nil {
		call := *part.ToolCall
		call.Arguments = cloneMap(part.ToolCall.Arguments)
		cloned.ToolCall = &call
	}

	if part.ToolResult != nil {
		result := *part.ToolResult
		cloned.ToolResult = &result
	}

	return cloned
}

func cloneGenerationConfig(cfg kit.GenerationConfig) kit.GenerationConfig {
	cloned := cfg

	if cfg.Temperature != nil {
		v := *cfg.Temperature
		cloned.Temperature = &v
	}

	if cfg.TopP != nil {
		v := *cfg.TopP
		cloned.TopP = &v
	}

	if cfg.MaxOutputTokens != nil {
		v := *cfg.MaxOutputTokens
		cloned.MaxOutputTokens = &v
	}

	return cloned
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = cloneAny(v)
	}

	return dst
}

func cloneSlice(src []any) []any {
	if src == nil {
		return nil
	}

	dst := make([]any, len(src))
	for i, v := range src {
		dst[i] = cloneAny(v)
	}

	return dst
}

func cloneAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return cloneMap(x)
	case []any:
		return cloneSlice(x)
	case []byte:
		return append([]byte(nil), x...)
	default:
		return x
	}
}

func cloneSchema(schema *jsonschema.Schema) *jsonschema.Schema {
	if schema == nil {
		return nil
	}

	data, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}

	var cloned jsonschema.Schema
	if err := json.Unmarshal(data, &cloned); err != nil {
		panic(err)
	}

	return &cloned
}
