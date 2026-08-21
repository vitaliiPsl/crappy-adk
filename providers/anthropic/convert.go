package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const emptyToolResultText = "(no output)"

func blank(text string) bool {
	return strings.TrimSpace(text) == ""
}

func convertRequestTools(tools []kit.Tool) []anthropicsdk.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	result := make([]anthropicsdk.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		def := tool.Definition()
		result = append(result, anthropicsdk.ToolUnionParam{
			OfTool: &anthropicsdk.ToolParam{
				Name:        def.Name,
				Description: anthropicsdk.String(def.Description),
				InputSchema: convertToolInputSchema(def.Schema),
			},
		})
	}

	return result
}

func convertToolInputSchema(schema map[string]any) anthropicsdk.ToolInputSchemaParam {
	result := anthropicsdk.ToolInputSchemaParam{}

	if props, ok := schema["properties"]; ok {
		result.Properties = props
	}

	if req, ok := schema["required"]; ok {
		if reqSlice, ok := req.([]any); ok {
			required := make([]string, 0, len(reqSlice))
			for _, r := range reqSlice {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}

			result.Required = required
		}
	}

	extras := make(map[string]any)
	for k, v := range schema {
		if k != "type" && k != "properties" && k != "required" {
			extras[k] = v
		}
	}

	if len(extras) > 0 {
		result.ExtraFields = extras
	}

	return result
}

func convertRequestMessages(messages []kit.Message) []anthropicsdk.MessageParam {
	reqMessages := make([]anthropicsdk.MessageParam, 0, len(messages))
	for _, msg := range messages {
		reqMessages = append(reqMessages, convertRequestMessage(msg)...)
	}

	return reqMessages
}

func convertRequestMessage(msg kit.Message) []anthropicsdk.MessageParam {
	items := convertRequestContentItems(msg.Content)
	if len(items) == 0 {
		return nil
	}

	switch msg.Role {
	case kit.RoleModel:
		return []anthropicsdk.MessageParam{anthropicsdk.NewAssistantMessage(items...)}
	default:
		return []anthropicsdk.MessageParam{anthropicsdk.NewUserMessage(items...)}
	}
}

func convertRequestContentItems(content []kit.Content) []anthropicsdk.ContentBlockParamUnion {
	items := make([]anthropicsdk.ContentBlockParamUnion, 0, len(content))
	for _, c := range content {
		if item, ok := convertRequestContentItem(c); ok {
			items = append(items, item)
		}
	}

	return items
}

func convertRequestContentItem(content kit.Content) (anthropicsdk.ContentBlockParamUnion, bool) {
	switch content.Type {
	case kit.ContentTypeText:
		if content.Text == nil || blank(content.Text.Text) {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		return anthropicsdk.NewTextBlock(content.Text.Text), true
	case kit.ContentTypeThinking:
		// Unlike the other providers, Anthropic cannot use a signature without the
		// thinking text, so a blank block is dropped even when it is signed.
		if content.Thinking == nil || blank(content.Thinking.Text) {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		t := content.Thinking

		return anthropicsdk.NewThinkingBlock(t.Signature, t.Text), true
	case kit.ContentTypeImage:
		return convertImageContent(content.Image)
	case kit.ContentTypeAudio:
		return fallbackContent(content)
	case kit.ContentTypeResource:
		return convertResourceContent(content.Resource)
	case kit.ContentTypeToolCall:
		if content.ToolCall == nil {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		tc := content.ToolCall

		return anthropicsdk.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name), true
	case kit.ContentTypeToolResult:
		if content.ToolResult == nil {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		tr := content.ToolResult

		return convertToolResultContent(tr), true
	case kit.ContentTypeSummary:
		if content.Summary == nil || blank(content.Summary.Text) {
			return anthropicsdk.ContentBlockParamUnion{}, false
		}

		return anthropicsdk.NewTextBlock(content.Summary.Text), true
	}

	return anthropicsdk.ContentBlockParamUnion{}, false
}

func convertImageContent(image *kit.Image) (anthropicsdk.ContentBlockParamUnion, bool) {
	if image == nil {
		return anthropicsdk.ContentBlockParamUnion{}, false
	}

	if len(image.Data) > 0 {
		return anthropicsdk.NewImageBlockBase64(image.MIMEType, base64.StdEncoding.EncodeToString(image.Data)), true
	}

	if image.URI != "" {
		return anthropicsdk.NewImageBlock(anthropicsdk.URLImageSourceParam{URL: image.URI}), true
	}

	return fallbackContent(kit.Content{Type: kit.ContentTypeImage, Image: image})
}

func convertResourceContent(resource *kit.Resource) (anthropicsdk.ContentBlockParamUnion, bool) {
	if resource == nil {
		return anthropicsdk.ContentBlockParamUnion{}, false
	}

	if resource.Text != "" {
		return anthropicsdk.NewDocumentBlock(anthropicsdk.PlainTextSourceParam{Data: resource.Text}), true
	}

	if strings.EqualFold(resource.MIMEType, "application/pdf") && len(resource.Blob) > 0 {
		return anthropicsdk.NewDocumentBlock(anthropicsdk.Base64PDFSourceParam{
			Data: base64.StdEncoding.EncodeToString(resource.Blob),
		}), true
	}

	if strings.EqualFold(resource.MIMEType, "application/pdf") && resource.URI != "" {
		return anthropicsdk.NewDocumentBlock(anthropicsdk.URLPDFSourceParam{URL: resource.URI}), true
	}

	if strings.HasPrefix(resource.MIMEType, "image/") {
		return convertImageContent(&kit.Image{
			MIMEType: resource.MIMEType,
			Data:     resource.Blob,
			URI:      resource.URI,
		})
	}

	return fallbackContent(kit.Content{Type: kit.ContentTypeResource, Resource: resource})
}

func fallbackContent(content kit.Content) (anthropicsdk.ContentBlockParamUnion, bool) {
	text, ok := kit.ContentText(content)
	if !ok || blank(text) {
		return anthropicsdk.ContentBlockParamUnion{}, false
	}

	return anthropicsdk.NewTextBlock(text), true
}

func convertToolResultContent(result *kit.ToolResult) anthropicsdk.ContentBlockParamUnion {
	content := convertToolResultItems(result.Output.Content)
	if len(content) == 0 {
		text := toolResultText(result)
		if blank(text) {
			text = emptyToolResultText
		}

		content = []anthropicsdk.ToolResultBlockParamContentUnion{{
			OfText: &anthropicsdk.TextBlockParam{Text: text},
		}}
	}

	return anthropicsdk.ContentBlockParamUnion{
		OfToolResult: &anthropicsdk.ToolResultBlockParam{
			ToolUseID: result.Call.ID,
			IsError:   anthropicsdk.Bool(result.Error != ""),
			Content:   content,
		},
	}
}

func convertToolResultItems(content []kit.Content) []anthropicsdk.ToolResultBlockParamContentUnion {
	items := make([]anthropicsdk.ToolResultBlockParamContentUnion, 0, len(content))
	for _, content := range content {
		if item, ok := convertToolResultItem(content); ok {
			items = append(items, item)
		}
	}

	return items
}

func convertToolResultItem(content kit.Content) (anthropicsdk.ToolResultBlockParamContentUnion, bool) {
	switch content.Type {
	case kit.ContentTypeText:
		if content.Text == nil || blank(content.Text.Text) {
			return anthropicsdk.ToolResultBlockParamContentUnion{}, false
		}

		return anthropicsdk.ToolResultBlockParamContentUnion{OfText: &anthropicsdk.TextBlockParam{Text: content.Text.Text}}, true
	case kit.ContentTypeImage:
		image, ok := convertToolResultImage(content.Image)

		return anthropicsdk.ToolResultBlockParamContentUnion{OfImage: image}, ok
	case kit.ContentTypeResource:
		if content.Resource == nil {
			return anthropicsdk.ToolResultBlockParamContentUnion{}, false
		}

		if strings.HasPrefix(content.Resource.MIMEType, "image/") {
			image, ok := convertToolResultImage(&kit.Image{
				MIMEType: content.Resource.MIMEType,
				Data:     content.Resource.Blob,
				URI:      content.Resource.URI,
			})

			return anthropicsdk.ToolResultBlockParamContentUnion{OfImage: image}, ok
		}

		document, ok := convertToolResultDocument(content.Resource)

		return anthropicsdk.ToolResultBlockParamContentUnion{OfDocument: document}, ok
	case kit.ContentTypeAudio:
		text, ok := kit.ContentText(content)
		if !ok || blank(text) {
			return anthropicsdk.ToolResultBlockParamContentUnion{}, false
		}

		return anthropicsdk.ToolResultBlockParamContentUnion{OfText: &anthropicsdk.TextBlockParam{Text: text}}, true
	default:
		text, ok := kit.ContentText(content)
		if !ok || blank(text) {
			return anthropicsdk.ToolResultBlockParamContentUnion{}, false
		}

		return anthropicsdk.ToolResultBlockParamContentUnion{OfText: &anthropicsdk.TextBlockParam{Text: text}}, true
	}
}

func convertToolResultImage(image *kit.Image) (*anthropicsdk.ImageBlockParam, bool) {
	if image == nil {
		return nil, false
	}

	out := &anthropicsdk.ImageBlockParam{}
	switch {
	case len(image.Data) > 0:
		out.Source.OfBase64 = &anthropicsdk.Base64ImageSourceParam{
			Data:      base64.StdEncoding.EncodeToString(image.Data),
			MediaType: anthropicsdk.Base64ImageSourceMediaType(image.MIMEType),
		}
	case image.URI != "":
		out.Source.OfURL = &anthropicsdk.URLImageSourceParam{URL: image.URI}
	default:
		return nil, false
	}

	return out, true
}

func convertToolResultDocument(resource *kit.Resource) (*anthropicsdk.DocumentBlockParam, bool) {
	if resource == nil {
		return nil, false
	}

	out := &anthropicsdk.DocumentBlockParam{}
	switch {
	case resource.Text != "":
		out.Source.OfText = &anthropicsdk.PlainTextSourceParam{Data: resource.Text}
	case strings.EqualFold(resource.MIMEType, "application/pdf") && len(resource.Blob) > 0:
		out.Source.OfBase64 = &anthropicsdk.Base64PDFSourceParam{Data: base64.StdEncoding.EncodeToString(resource.Blob)}
	case strings.EqualFold(resource.MIMEType, "application/pdf") && resource.URI != "":
		out.Source.OfURL = &anthropicsdk.URLPDFSourceParam{URL: resource.URI}
	default:
		return nil, false
	}

	return out, true
}

func toolResultText(result *kit.ToolResult) string {
	if result.Error != "" {
		return result.Error
	}

	if text := kit.ContentsText(result.Output.Content); text != "" {
		return text
	}

	if result.Output.Structured == nil {
		return ""
	}

	data, err := json.Marshal(result.Output.Structured)
	if err != nil {
		return ""
	}

	return string(data)
}

func convertResponse(resp *anthropicsdk.Message) kit.ModelResponse {
	content := make([]kit.Content, 0, len(resp.Content))

	for _, block := range resp.Content {
		content = append(content, convertResponseContent(block)...)
	}

	return kit.ModelResponse{
		Message:      kit.NewModelMessage(content...),
		FinishReason: convertResponseFinishReason(resp),
		Usage:        convertResponseUsage(resp),
	}
}

func convertResponseContent(block anthropicsdk.ContentBlockUnion) []kit.Content {
	switch block.Type {
	case "text":
		return []kit.Content{kit.NewTextContent(block.Text)}

	case "thinking":
		return []kit.Content{kit.NewThinkingContent("", block.Thinking, block.Signature)}

	case "tool_use":
		if tc, ok := convertResponseToolCall(block); ok {
			return []kit.Content{kit.NewToolCallContent(tc)}
		}
	}

	return nil
}

func convertResponseToolCall(block anthropicsdk.ContentBlockUnion) (kit.ToolCall, bool) {
	var args map[string]any
	if err := json.Unmarshal(block.Input, &args); err != nil {
		return kit.ToolCall{}, false
	}

	return kit.ToolCall{
		ID:        block.ID,
		Name:      block.Name,
		Arguments: args,
	}, true
}

func convertResponseFinishReason(resp *anthropicsdk.Message) kit.FinishReason {
	switch resp.StopReason {
	case anthropicsdk.StopReasonEndTurn:
		return kit.FinishReasonStop
	case anthropicsdk.StopReasonMaxTokens:
		return kit.FinishReasonMaxTokens
	case anthropicsdk.StopReasonToolUse:
		return kit.FinishReasonToolCall
	case anthropicsdk.StopReasonRefusal:
		return kit.FinishReasonSafety
	default:
		return kit.FinishReasonUnknown
	}
}

func convertResponseUsage(resp *anthropicsdk.Message) kit.Usage {
	return kit.Usage{
		InputTokens:      resp.Usage.InputTokens,
		OutputTokens:     resp.Usage.OutputTokens,
		CacheReadTokens:  resp.Usage.CacheReadInputTokens,
		CacheWriteTokens: resp.Usage.CacheCreationInputTokens,
	}
}
