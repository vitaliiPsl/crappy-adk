package openai

import (
	"context"

	"github.com/openai/openai-go/v3/responses"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func newStream(ctx context.Context, model *Model, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.ModelEvent]) (kit.ModelResponse, error) {
		req := buildRequestParams(model.id, request)

		stream := model.client.Responses.NewStreaming(ctx, req)
		defer func() {
			_ = stream.Close()
		}()

		var (
			result        kit.ModelResponse
			outputContent []kit.Content
			contentEvents []kit.Content
		)
		for stream.Next() {
			event := stream.Current().AsAny()

			switch e := event.(type) {
			case responses.ResponseCompletedEvent:
				result = convertResponse(&e.Response)
			case responses.ResponseOutputItemDoneEvent:
				outputContent = append(outputContent, convertResponseContent(e.Item)...)

				if err := handleStreamEvent(event, emit); err != nil {
					return result, err
				}
			case responses.ResponseTextDoneEvent:
				contentEvents = append(contentEvents, kit.NewTextContent(e.Text))

				if err := handleStreamEvent(event, emit); err != nil {
					return result, err
				}
			case responses.ResponseReasoningSummaryTextDoneEvent:
				contentEvents = append(contentEvents, kit.NewThinkingContent(e.ItemID, e.Text, ""))

				if err := handleStreamEvent(event, emit); err != nil {
					return result, err
				}
			default:
				if err := handleStreamEvent(event, emit); err != nil {
					return result, err
				}
			}
		}

		if err := stream.Err(); err != nil {
			return result, mapError(err)
		}

		return finalizeStreamResponse(result, outputContent, contentEvents), nil
	})
}

func finalizeStreamResponse(result kit.ModelResponse, outputContent, contentEvents []kit.Content) kit.ModelResponse {
	if len(result.Message.Content) > 0 {
		return result
	}

	if len(outputContent) > 0 {
		result.Message = kit.NewModelMessage(outputContent...)
	} else if len(contentEvents) > 0 {
		result.Message = kit.NewModelMessage(contentEvents...)
	}

	if len(result.Message.ToolCalls()) > 0 {
		result.FinishReason = kit.FinishReasonToolCall
	}

	return result
}

func handleStreamEvent(
	event any,
	emit kit.Emitter[kit.ModelEvent],
) error {
	switch e := event.(type) {
	case responses.ResponseContentPartAddedEvent:
		return emitStreamContentStarted(e, emit)
	case responses.ResponseTextDeltaEvent:
		return emit.Emit(kit.NewModelContentDeltaEvent(kit.NewTextContent(e.Delta)))
	case responses.ResponseTextDoneEvent:
		return emit.Emit(kit.NewModelContentDoneEvent(kit.NewTextContent(e.Text)))
	case responses.ResponseReasoningSummaryPartAddedEvent:
		return emit.Emit(kit.NewModelContentStartedEvent(kit.NewThinkingContent(e.ItemID, e.Part.Text, "")))
	case responses.ResponseReasoningSummaryTextDeltaEvent:
		return emit.Emit(kit.NewModelContentDeltaEvent(kit.NewThinkingContent(e.ItemID, e.Delta, "")))
	case responses.ResponseReasoningSummaryTextDoneEvent:
		return emit.Emit(kit.NewModelContentDoneEvent(kit.NewThinkingContent(e.ItemID, e.Text, "")))
	case responses.ResponseOutputItemAddedEvent:
		return emitOutputItemStarted(e, emit)
	case responses.ResponseOutputItemDoneEvent:
		return emitOutputItemDone(e, emit)
	case responses.ResponseErrorEvent:
		return mapStreamError(e.Code, e.Message)
	default:
		return nil
	}
}

func emitStreamContentStarted(
	event responses.ResponseContentPartAddedEvent,
	emit kit.Emitter[kit.ModelEvent],
) error {
	if event.Part.Type != "output_text" {
		return nil
	}

	return emit.Emit(kit.NewModelContentStartedEvent(kit.NewTextContent(event.Part.Text)))
}

func emitOutputItemStarted(
	event responses.ResponseOutputItemAddedEvent,
	emit kit.Emitter[kit.ModelEvent],
) error {
	if _, ok := event.Item.AsAny().(responses.ResponseFunctionToolCall); !ok {
		return nil
	}

	for _, content := range convertResponseContent(event.Item) {
		if err := emit.Emit(kit.NewModelContentStartedEvent(content)); err != nil {
			return err
		}
	}

	return nil
}

func emitOutputItemDone(
	event responses.ResponseOutputItemDoneEvent,
	emit kit.Emitter[kit.ModelEvent],
) error {
	if _, ok := event.Item.AsAny().(responses.ResponseFunctionToolCall); !ok {
		return nil
	}

	for _, content := range convertResponseContent(event.Item) {
		if err := emit.Emit(kit.NewModelContentDoneEvent(content)); err != nil {
			return err
		}
	}

	return nil
}
