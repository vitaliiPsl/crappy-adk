package kit

import "context"

// RunContext is the per-run state that the agent threads through its loop.
// Hooks receive a pointer to it and may inspect it or mutate memory.
type RunContext struct {
	context.Context

	// Memory is the memory configured for this run.
	Memory Memory

	// Messages holds every message produced during this run, in order.
	// It is returned to the caller in [AgentResponse.Messages].
	Messages []Message

	// Usage is the cumulative token usage across all completed model calls.
	Usage Usage

	// LastUsage is the usage reported by the most recent model response.
	LastUsage Usage

	// Events emits agent stream events. It is a no-op for non-streaming runs.
	Events Emitter[AgentEvent]
}

// Append records a message in the append-only log returned to the caller.
func (rc *RunContext) Append(msg Message) {
	rc.Messages = append(rc.Messages, msg)
}

// RecordUsage stores u as the most recent model call's usage and adds it to the
// cumulative total.
func (rc *RunContext) RecordUsage(u Usage) {
	rc.LastUsage = u
	rc.Usage.Add(u)
}

// Response returns the agent response accumulated so far. Output is taken only
// from the latest run message when that message is a model message.
func (rc *RunContext) Response() AgentResponse {
	resp := AgentResponse{
		Messages:  rc.Messages,
		Usage:     rc.Usage,
		LastUsage: rc.LastUsage,
	}

	if len(rc.Messages) == 0 {
		return resp
	}

	last := rc.Messages[len(rc.Messages)-1]
	if last.Role == RoleModel {
		resp.Output = last.TextContent()
	}

	return resp
}

// Emit emits an agent stream event if an emitter is configured.
func (rc *RunContext) Emit(event AgentEvent) error {
	if rc.Events == nil {
		return nil
	}

	return rc.Events.Emit(event)
}
