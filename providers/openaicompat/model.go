package openaicompat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type model struct {
	client *openaisdk.Client
	config kit.ModelConfig
}

var _ kit.Model = (*model)(nil)

func (m *model) Config() kit.ModelConfig {
	return m.config
}

func (m *model) Generate(ctx context.Context, req kit.ModelRequest) (kit.ModelResponse, error) {
	params := m.buildParams(req)

	resp, err := m.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return kit.ModelResponse{}, convertError(err)
	}

	return convertResponse(*resp), nil
}

func (m *model) GenerateStream(ctx context.Context, req kit.ModelRequest) (*kit.Stream[kit.ModelEvent, kit.ModelResponse], error) {
	params := m.buildParams(req)
	params.StreamOptions = openaisdk.ChatCompletionStreamOptionsParam{
		IncludeUsage: openaisdk.Bool(true),
	}

	stream := m.client.Chat.Completions.NewStreaming(ctx, params)

	return kit.NewStream(func(yield func(kit.ModelEvent, error) bool) kit.ModelResponse {
		return streamResponse(stream, yield)
	}), nil
}

func (m *model) buildParams(req kit.ModelRequest) openaisdk.ChatCompletionNewParams {
	params := openaisdk.ChatCompletionNewParams{
		Model:    m.config.ID,
		Messages: convertMessages(req.Instruction, req.Messages),
		Tools:    convertTools(req.Tools),
	}

	gc := req.Config

	if gc.Temperature != nil {
		params.Temperature = openaisdk.Float(float64(*gc.Temperature))
	}

	if gc.TopP != nil {
		params.TopP = openaisdk.Float(float64(*gc.TopP))
	}

	if gc.MaxOutputTokens != nil {
		params.MaxCompletionTokens = openaisdk.Int(int64(*gc.MaxOutputTokens))
	}

	return params
}

func streamResponse(
	stream *ssestream.Stream[openaisdk.ChatCompletionChunk],
	yield func(kit.ModelEvent, error) bool,
) kit.ModelResponse {
	defer func() { _ = stream.Close() }()

	type partialToolCall struct {
		id        string
		name      string
		arguments strings.Builder
	}

	partials := map[int64]*partialToolCall{}

	var (
		content            strings.Builder
		usage              openaisdk.CompletionUsage
		contentPartStarted bool
	)

	for stream.Next() {
		chunk := stream.Current()

		if chunk.Usage.TotalTokens > 0 {
			usage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			delta := choice.Delta

			if delta.Content != "" {
				content.WriteString(delta.Content)

				if !contentPartStarted {
					contentPartStarted = true

					if !yield(kit.NewModelContentPartStartedEvent(kit.ContentTypeText), nil) {
						return kit.ModelResponse{}
					}
				}

				if !yield(kit.NewModelContentPartDeltaEvent(kit.ContentTypeText, delta.Content), nil) {
					return kit.ModelResponse{}
				}
			}

			for _, tcd := range delta.ToolCalls {
				p, ok := partials[tcd.Index]
				if !ok {
					p = &partialToolCall{}
					partials[tcd.Index] = p
				}

				if tcd.ID != "" {
					p.id = tcd.ID
				}

				if tcd.Function.Name != "" {
					p.name = tcd.Function.Name
				}

				p.arguments.WriteString(tcd.Function.Arguments)
			}
		}
	}

	if err := stream.Err(); err != nil {
		yield(kit.ModelEvent{}, convertError(err))

		return kit.ModelResponse{}
	}

	toolCalls := make([]kit.ToolCall, 0, len(partials))

	for i := range int64(len(partials)) {
		p, ok := partials[i]
		if !ok {
			continue
		}

		tc, err := assembleToolCall(p.id, p.name, p.arguments.String())
		if err != nil {
			yield(kit.ModelEvent{}, err)

			return kit.ModelResponse{}
		}

		if !yield(kit.NewModelToolCallStartedEvent(tc), nil) {
			return kit.ModelResponse{}
		}

		if !yield(kit.NewModelToolCallDoneEvent(tc), nil) {
			return kit.ModelResponse{}
		}

		toolCalls = append(toolCalls, tc)
	}

	finishReason := kit.FinishReasonStop
	if len(toolCalls) > 0 {
		finishReason = kit.FinishReasonToolCall
	}

	resp := kit.ModelResponse{
		Message:      kit.NewAssistantMessage(content.String(), "", toolCalls),
		FinishReason: finishReason,
		Usage: kit.Usage{
			InputTokens:  int32(usage.PromptTokens),
			OutputTokens: int32(usage.CompletionTokens),
		},
	}

	for _, part := range resp.Message.Content {
		if !yield(kit.NewModelContentPartDoneEvent(part), nil) {
			return resp
		}
	}

	return resp
}

func convertMessages(instruction string, msgs []kit.Message) []openaisdk.ChatCompletionMessageParamUnion {
	out := make([]openaisdk.ChatCompletionMessageParamUnion, 0, len(msgs)+1)

	if instruction != "" {
		out = append(out, openaisdk.SystemMessage(instruction))
	}

	for _, msg := range msgs {
		out = append(out, convertMessage(msg)...)
	}

	return out
}

func convertMessage(msg kit.Message) []openaisdk.ChatCompletionMessageParamUnion {
	switch msg.Role {
	case kit.MessageRoleUser:
		return []openaisdk.ChatCompletionMessageParamUnion{
			openaisdk.UserMessage(convertUserContent(msg.Content)),
		}

	case kit.MessageRoleAssistant:
		param := openaisdk.ChatCompletionAssistantMessageParam{}

		if text := msg.Text(); text != "" {
			param.Content = openaisdk.ChatCompletionAssistantMessageParamContentUnion{
				OfString: openaisdk.String(text),
			}
		}

		for _, tc := range msg.ToolCalls {
			args, _ := json.Marshal(tc.Arguments)
			param.ToolCalls = append(param.ToolCalls, openaisdk.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openaisdk.ChatCompletionMessageFunctionToolCallParam{
					ID: tc.ID,
					Function: openaisdk.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: string(args),
					},
				},
			})
		}

		return []openaisdk.ChatCompletionMessageParamUnion{
			{OfAssistant: &param},
		}

	case kit.MessageRoleTool:
		return []openaisdk.ChatCompletionMessageParamUnion{
			openaisdk.ToolMessage(msg.Text(), msg.ToolCallID),
		}

	default:
		return []openaisdk.ChatCompletionMessageParamUnion{
			openaisdk.UserMessage(msg.Text()),
		}
	}
}

func convertUserContent(parts []kit.ContentPart) []openaisdk.ChatCompletionContentPartUnionParam {
	if len(parts) == 0 {
		return nil
	}

	out := make([]openaisdk.ChatCompletionContentPartUnionParam, 0, len(parts))
	for _, part := range parts {
		if converted, ok := convertContentPart(part); ok {
			out = append(out, converted)
		}
	}

	return out
}

func convertContentPart(part kit.ContentPart) (openaisdk.ChatCompletionContentPartUnionParam, bool) {
	switch part.Type {
	case kit.ContentTypeText:
		return openaisdk.TextContentPart(part.Text), true

	case kit.ContentTypeImage:
		imageURL := part.URL
		if len(part.Data) > 0 {
			imageURL = "data:" + part.MediaType + ";base64," + base64.StdEncoding.EncodeToString(part.Data)
		}

		if imageURL == "" {
			return openaisdk.ChatCompletionContentPartUnionParam{}, false
		}

		return openaisdk.ImageContentPart(openaisdk.ChatCompletionContentPartImageImageURLParam{
			URL: imageURL,
		}), true

	case kit.ContentTypeDocument:
		fileData, ok := documentFileData(part)
		if !ok {
			return openaisdk.ChatCompletionContentPartUnionParam{}, false
		}

		return openaisdk.FileContentPart(openaisdk.ChatCompletionContentPartFileFileParam{
			FileData: openaisdk.String(fileData),
			Filename: openaisdk.String(documentFilename(part)),
		}), true
	}

	return openaisdk.ChatCompletionContentPartUnionParam{}, false
}

func documentFileData(part kit.ContentPart) (string, bool) {
	if len(part.Data) > 0 {
		return base64.StdEncoding.EncodeToString(part.Data), true
	}

	if strings.HasPrefix(part.URL, "data:") {
		_, data, ok := strings.Cut(part.URL, ",")
		if ok {
			return data, true
		}
	}

	return "", false
}

func documentFilename(part kit.ContentPart) string {
	if part.URL != "" && !strings.HasPrefix(part.URL, "data:") {
		rawPath := part.URL
		if parsed, err := url.Parse(part.URL); err == nil && parsed.Path != "" {
			rawPath = parsed.Path
		}

		name := path.Base(rawPath)
		if name != "" && name != "." && name != "/" {
			return name
		}
	}

	switch part.MediaType {
	case "application/pdf":
		return "document.pdf"
	default:
		return "document"
	}
}

func convertTools(tools []kit.ToolDefinition) []openaisdk.ChatCompletionToolUnionParam {
	if len(tools) == 0 {
		return nil
	}

	out := make([]openaisdk.ChatCompletionToolUnionParam, 0, len(tools))

	for _, tool := range tools {
		schema, err := json.Marshal(tool.Schema)
		if err != nil {
			continue
		}

		var parameters map[string]any
		if err := json.Unmarshal(schema, &parameters); err != nil {
			continue
		}

		out = append(out, openaisdk.ChatCompletionToolUnionParam{
			OfFunction: &openaisdk.ChatCompletionFunctionToolParam{
				Function: openaisdk.FunctionDefinitionParam{
					Name:        tool.Name,
					Description: openaisdk.String(tool.Description),
					Parameters:  openaisdk.FunctionParameters(parameters),
				},
			},
		})
	}

	return out
}

func convertResponse(resp openaisdk.ChatCompletion) kit.ModelResponse {
	if len(resp.Choices) == 0 {
		return kit.ModelResponse{}
	}

	choice := resp.Choices[0]
	msg := choice.Message

	toolCalls := make([]kit.ToolCall, 0, len(msg.ToolCalls))

	for _, tc := range msg.ToolCalls {
		assembled, err := assembleToolCall(tc.ID, tc.Function.Name, tc.Function.Arguments)
		if err != nil {
			continue
		}

		toolCalls = append(toolCalls, assembled)
	}

	return kit.ModelResponse{
		Message:      kit.NewAssistantMessage(msg.Content, "", toolCalls),
		FinishReason: convertFinishReason(choice.FinishReason),
		Usage: kit.Usage{
			InputTokens:  int32(resp.Usage.PromptTokens),
			OutputTokens: int32(resp.Usage.CompletionTokens),
		},
	}
}

func assembleToolCall(id, name, argsJSON string) (kit.ToolCall, error) {
	var args map[string]any

	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return kit.ToolCall{}, fmt.Errorf("openaicompat: unmarshal tool args for %q: %w", name, err)
		}
	}

	return kit.ToolCall{ID: id, Name: name, Arguments: args}, nil
}

func convertFinishReason(reason string) kit.FinishReason {
	switch reason {
	case "stop":
		return kit.FinishReasonStop
	case "tool_calls":
		return kit.FinishReasonToolCall
	case "length":
		return kit.FinishReasonMaxTokens
	case "content_filter":
		return kit.FinishReasonSafety
	default:
		return kit.FinishReasonUnknown
	}
}
