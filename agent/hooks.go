package agent

import (
	"github.com/vitaliiPsl/crappy-adk/kit"
)

type hooks struct {
	turnStart     []kit.OnTurnStart
	turnEnd       []kit.OnTurnEnd
	modelRequest  []kit.OnModelRequest
	modelResponse []kit.OnModelResponse
	toolCall      []kit.OnToolCall
	toolResult    []kit.OnToolResult
}

func (h *hooks) onTurnStart(rc *kit.RunContext) error {
	for _, fn := range h.turnStart {
		if err := fn(rc); err != nil {
			return err
		}
	}

	return nil
}

func (h *hooks) onTurnEnd(rc *kit.RunContext) error {
	for _, fn := range h.turnEnd {
		if err := fn(rc); err != nil {
			return err
		}
	}

	return nil
}

func (h *hooks) onModelRequest(rc *kit.RunContext, req kit.ModelRequest) (kit.ModelRequest, error) {
	for _, fn := range h.modelRequest {
		var err error

		req, err = fn(rc, req)
		if err != nil {
			return kit.ModelRequest{}, err
		}
	}

	return req, nil
}

func (h *hooks) onModelResponse(rc *kit.RunContext, resp kit.ModelResponse) (kit.ModelResponse, error) {
	for _, fn := range h.modelResponse {
		var err error

		resp, err = fn(rc, resp)
		if err != nil {
			return kit.ModelResponse{}, err
		}
	}

	return resp, nil
}

func (h *hooks) onToolCall(rc *kit.RunContext, call kit.ToolCall) (kit.ToolCall, error) {
	for _, fn := range h.toolCall {
		next, err := fn(rc, call)
		if err != nil {
			return call, err
		}

		call = next
	}

	return call, nil
}

func (h *hooks) onToolResult(rc *kit.RunContext, result kit.ToolResult) (kit.ToolResult, error) {
	for _, fn := range h.toolResult {
		next, err := fn(rc, result)
		if err != nil {
			return result, err
		}

		result = next
	}

	return result, nil
}
