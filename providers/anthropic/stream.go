package anthropic

import (
	"context"
	"encoding/json"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func newStream(ctx context.Context, model *Model, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.ModelEvent]) (kit.ModelResponse, error) {
		params := buildRequestParams(model.id, request)

		stream := model.client.Messages.NewStreaming(ctx, params)
		defer func() { _ = stream.Close() }()

		var acc anthropicsdk.Message
		for stream.Next() {
			event := stream.Current()

			if err := acc.Accumulate(event); err != nil {
				return kit.ModelResponse{}, err
			}

			if err := handleStreamEvent(event, &acc, emit); err != nil {
				return kit.ModelResponse{}, err
			}
		}

		if err := stream.Err(); err != nil {
			return kit.ModelResponse{}, mapError(err)
		}

		return convertResponse(&acc), nil
	})
}

func handleStreamEvent(
	event anthropicsdk.MessageStreamEventUnion,
	acc *anthropicsdk.Message,
	emit kit.Emitter[kit.ModelEvent],
) error {
	switch e := event.AsAny().(type) {
	case anthropicsdk.ContentBlockStartEvent:
		return emitContentStarted(e, emit)
	case anthropicsdk.ContentBlockDeltaEvent:
		return emitContentDelta(e, emit)
	case anthropicsdk.ContentBlockStopEvent:
		return emitContentDone(e, acc, emit)
	}

	return nil
}

func emitContentStarted(e anthropicsdk.ContentBlockStartEvent, emit kit.Emitter[kit.ModelEvent]) error {
	switch block := e.ContentBlock.AsAny().(type) {
	case anthropicsdk.TextBlock:
		_ = block

		return emit.Emit(kit.NewModelContentStartedEvent(kit.NewTextContent("")))
	case anthropicsdk.ThinkingBlock:
		_ = block

		return emit.Emit(kit.NewModelContentStartedEvent(kit.NewThinkingContent("", "", "")))
	case anthropicsdk.ToolUseBlock:
		return emit.Emit(kit.NewModelContentStartedEvent(kit.NewToolCallContent(kit.ToolCall{
			ID:   block.ID,
			Name: block.Name,
		})))
	}

	return nil
}

func emitContentDelta(e anthropicsdk.ContentBlockDeltaEvent, emit kit.Emitter[kit.ModelEvent]) error {
	switch delta := e.Delta.AsAny().(type) {
	case anthropicsdk.TextDelta:
		return emit.Emit(kit.NewModelContentDeltaEvent(kit.NewTextContent(delta.Text)))
	case anthropicsdk.ThinkingDelta:
		return emit.Emit(kit.NewModelContentDeltaEvent(kit.NewThinkingContent("", delta.Thinking, "")))
	}

	return nil
}

func emitContentDone(e anthropicsdk.ContentBlockStopEvent, acc *anthropicsdk.Message, emit kit.Emitter[kit.ModelEvent]) error {
	if int(e.Index) >= len(acc.Content) {
		return nil
	}

	block := acc.Content[e.Index]
	switch b := block.AsAny().(type) {
	case anthropicsdk.TextBlock:
		return emit.Emit(kit.NewModelContentDoneEvent(kit.NewTextContent(b.Text)))
	case anthropicsdk.ThinkingBlock:
		return emit.Emit(kit.NewModelContentDoneEvent(kit.NewThinkingContent("", b.Thinking, "")))
	case anthropicsdk.ToolUseBlock:
		var args map[string]any
		if err := json.Unmarshal(b.Input, &args); err != nil {
			return err
		}

		return emit.Emit(kit.NewModelContentDoneEvent(kit.NewToolCallContent(kit.ToolCall{
			ID:        b.ID,
			Name:      b.Name,
			Arguments: args,
		})))
	}

	return nil
}
