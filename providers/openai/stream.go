package openai

import (
	"context"

	"github.com/openai/openai-go/v3/responses"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func newStream(ctx context.Context, model *Model, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.ModelEvent]) (kit.ModelResponse, error) {
		req, err := buildRequestParams(model.id, request)
		if err != nil {
			return kit.ModelResponse{}, err
		}

		stream := model.client.Responses.NewStreaming(ctx, req)
		defer func() {
			_ = stream.Close()
		}()

		var result kit.ModelResponse
		for stream.Next() {
			event := stream.Current().AsAny()

			switch e := event.(type) {
			case responses.ResponseCompletedEvent:
				result = convertResponse(&e.Response)
			default:
				if err := handleStreamEvent(event, emit); err != nil {
					return result, err
				}
			}
		}

		if err := stream.Err(); err != nil {
			return result, mapError(err)
		}

		return result, nil
	})
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
