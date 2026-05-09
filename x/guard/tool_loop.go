package guard

import (
	"encoding/json"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

// WithRepeatedToolCallLimit limits identical tool calls within a rolling window
// of tool-calling model responses. Tool call IDs are ignored; only tool names
// and arguments are compared. The window includes the current model response.
// A non-positive maxRepeats disables loop detection. A non-positive window
// checks all prior tool-calling model responses.
func WithRepeatedToolCallLimit(maxRepeats, window int) agent.Option {
	if maxRepeats <= 0 {
		return agent.WithExtensions()
	}

	return agent.WithOnModelResponse(func(rc *kit.RunContext, resp kit.ModelResponse) (kit.ModelResponse, error) {
		calls := resp.Message.ToolCalls()
		if len(calls) == 0 {
			return resp, nil
		}

		counts := make(map[string]int, len(calls))
		for _, call := range calls {
			key := toolCallFingerprint(call)

			counts[key]++
			if counts[key] > maxRepeats {
				return kit.ModelResponse{}, fmt.Errorf("%w: repeated tool call %d times", ErrToolLoopDetected, counts[key])
			}
		}

		previousToolResponses := 0
		for i := len(rc.Generated) - 1; i >= 0; i-- {
			msg := rc.Generated[i]
			if msg.Role != kit.RoleModel {
				continue
			}

			previousCalls := msg.ToolCalls()
			if len(previousCalls) == 0 {
				continue
			}

			if window > 0 && previousToolResponses >= window-1 {
				break
			}

			previousToolResponses++

			for _, call := range previousCalls {
				key := toolCallFingerprint(call)
				if _, ok := counts[key]; !ok {
					continue
				}

				counts[key]++
				if counts[key] > maxRepeats {
					return kit.ModelResponse{}, fmt.Errorf("%w: repeated tool call %d times", ErrToolLoopDetected, counts[key])
				}
			}
		}

		return resp, nil
	})
}

type toolCallKey struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func toolCallFingerprint(call kit.ToolCall) string {
	key := toolCallKey{
		Name:      call.Name,
		Arguments: call.Arguments,
	}

	data, err := json.Marshal(key)
	if err != nil {
		return fmt.Sprintf("%#v", key)
	}

	return string(data)
}
