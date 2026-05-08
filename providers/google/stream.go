package google

import (
	"context"

	"google.golang.org/genai"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type contentKind int

const (
	kindNone contentKind = iota
	kindText
	kindThinking
)

// streamAcc tracks in-flight state across streaming chunks.
type streamAcc struct {
	kind     contentKind
	textBuf  string
	thinkBuf string
	thinkSig []byte
	tools    []kit.Content
	lastResp *genai.GenerateContentResponse
}

func newStream(ctx context.Context, model *Model, request kit.ModelRequest) *kit.Stream[kit.ModelEvent, kit.ModelResponse] {
	return kit.NewStream(func(emit kit.Emitter[kit.ModelEvent]) (kit.ModelResponse, error) {
		contents, config := buildRequestParams(request)

		var acc streamAcc
		for resp, err := range model.client.Models.GenerateContentStream(ctx, model.id, contents, config) {
			if err != nil {
				return kit.ModelResponse{}, mapError(err)
			}

			if err := acc.handleChunk(resp, emit); err != nil {
				return kit.ModelResponse{}, err
			}
		}

		if err := acc.flush(emit); err != nil {
			return kit.ModelResponse{}, err
		}

		return acc.result(), nil
	})
}

func (a *streamAcc) handleChunk(resp *genai.GenerateContentResponse, emit kit.Emitter[kit.ModelEvent]) error {
	a.lastResp = resp

	if len(resp.Candidates) == 0 {
		return nil
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return nil
	}

	for _, part := range candidate.Content.Parts {
		if err := a.handlePart(part, emit); err != nil {
			return err
		}
	}

	return nil
}

func (a *streamAcc) handlePart(part *genai.Part, emit kit.Emitter[kit.ModelEvent]) error {
	switch {
	case part.FunctionCall != nil:
		if err := a.transitionTo(kindNone, emit); err != nil {
			return err
		}

		tc, _ := convertResponseToolCall(part.FunctionCall)
		content := kit.NewToolCallContent(tc)

		if err := emit.Emit(kit.NewModelContentStartedEvent(content)); err != nil {
			return err
		}

		if err := emit.Emit(kit.NewModelContentDoneEvent(content)); err != nil {
			return err
		}

		a.tools = append(a.tools, content)

	case part.Thought:
		if err := a.transitionTo(kindThinking, emit); err != nil {
			return err
		}

		a.thinkBuf += part.Text
		if len(part.ThoughtSignature) > 0 {
			a.thinkSig = part.ThoughtSignature
		}

		return emit.Emit(kit.NewModelContentDeltaEvent(kit.NewThinkingContent("", part.Text, "")))

	case part.Text != "":
		if err := a.transitionTo(kindText, emit); err != nil {
			return err
		}

		a.textBuf += part.Text

		return emit.Emit(kit.NewModelContentDeltaEvent(kit.NewTextContent(part.Text)))
	}

	return nil
}

// transitionTo emits Done for the current block and Started for the new kind.
func (a *streamAcc) transitionTo(next contentKind, emit kit.Emitter[kit.ModelEvent]) error {
	if a.kind == next {
		return nil
	}

	if err := a.emitDone(emit); err != nil {
		return err
	}

	a.kind = next

	switch next {
	case kindText:
		return emit.Emit(kit.NewModelContentStartedEvent(kit.NewTextContent("")))
	case kindThinking:
		return emit.Emit(kit.NewModelContentStartedEvent(kit.NewThinkingContent("", "", "")))
	}

	return nil
}

// emitDone emits a Done event for the current in-flight block.
func (a *streamAcc) emitDone(emit kit.Emitter[kit.ModelEvent]) error {
	switch a.kind {
	case kindText:
		return emit.Emit(kit.NewModelContentDoneEvent(kit.NewTextContent(a.textBuf)))
	case kindThinking:
		return emit.Emit(kit.NewModelContentDoneEvent(kit.NewThinkingContent("", a.thinkBuf, "")))
	}

	return nil
}

// flush closes the last open block after all chunks have been processed.
func (a *streamAcc) flush(emit kit.Emitter[kit.ModelEvent]) error {
	return a.emitDone(emit)
}

func (a *streamAcc) result() kit.ModelResponse {
	content := make([]kit.Content, 0)

	if a.thinkBuf != "" {
		content = append(content, kit.NewThinkingContent("", a.thinkBuf, string(a.thinkSig)))
	}

	if a.textBuf != "" {
		content = append(content, kit.NewTextContent(a.textBuf))
	}

	content = append(content, a.tools...)

	finishReason := kit.FinishReasonUnknown
	if len(a.tools) > 0 {
		finishReason = kit.FinishReasonToolCall
	} else if a.lastResp != nil && len(a.lastResp.Candidates) > 0 {
		switch a.lastResp.Candidates[0].FinishReason {
		case genai.FinishReasonStop:
			finishReason = kit.FinishReasonStop
		case genai.FinishReasonMaxTokens:
			finishReason = kit.FinishReasonMaxTokens
		case genai.FinishReasonSafety:
			finishReason = kit.FinishReasonSafety
		}
	}

	var usage kit.Usage
	if a.lastResp != nil {
		usage = convertResponseUsage(a.lastResp)
	}

	return kit.ModelResponse{
		Message:      kit.NewModelMessage(content),
		FinishReason: finishReason,
		Usage:        usage,
	}
}
