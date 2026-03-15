package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const (
	delegateToolName        = "delegate"
	delegateToolDescription = "Delegate a task to a specialized subagent and return its result."
)

// SubAgent defines a named agent that can be delegated to by a parent agent.
type SubAgent struct {
	// Name is the identifier the parent agent uses to select this subagent.
	Name string
	// Description explains what this subagent does. The parent agent uses this
	// to decide which subagent to delegate to.
	Description string
	// Agent is the underlying agent that runs when this subagent is selected.
	Agent *kit.Agent
}

type delegateInput struct {
	Agent string `json:"agent" jsonschema:"The subagent to delegate to"`
	Task  string `json:"task"  jsonschema:"A full description of the task for the subagent to complete"`
}

// WithSubAgents registers a set of subagents on the parent agent as a single
// "delegate" tool. The parent calls delegate with an agent name and task;
// the selected subagent runs its own full loop and returns result.
func WithSubAgents(subAgents ...SubAgent) kit.AgentOption {
	return func(a *kit.Agent) error {
		if len(subAgents) == 0 {
			return fmt.Errorf("WithSubAgents: at least one subagent is required")
		}

		registry := make(map[string]SubAgent, len(subAgents))
		names := make([]string, 0, len(subAgents))
		descParts := make([]string, 0, len(subAgents))

		for _, sa := range subAgents {
			if sa.Name == "" {
				return fmt.Errorf("WithSubAgents: subagent name cannot be empty")
			}

			if sa.Agent == nil {
				return fmt.Errorf("WithSubAgents: subagent %q has a nil agent", sa.Name)
			}

			if _, exists := registry[sa.Name]; exists {
				return fmt.Errorf("WithSubAgents: duplicate subagent name: %q", sa.Name)
			}

			registry[sa.Name] = sa
			names = append(names, sa.Name)
			descParts = append(descParts, fmt.Sprintf("%s: %s", sa.Name, sa.Description))
		}

		validNames := strings.Join(names, ", ")
		description := fmt.Sprintf("%s\n\nAvailable subagents:\n%s", delegateToolDescription, strings.Join(descParts, "\n"))

		t := MustFunction(
			delegateToolName,
			description,
			func(ctx context.Context, input delegateInput) (string, error) {
				sa, ok := registry[input.Agent]
				if !ok {
					return "", fmt.Errorf("unknown agent %q, valid agents are: %s", input.Agent, validNames)
				}

				resp, err := sa.Agent.Run(ctx, []kit.Message{
					kit.NewUserMessage(input.Task),
				})
				if err != nil {
					return "", fmt.Errorf("subagent %q failed: %w", input.Agent, err)
				}

				return resp.LastMessage().Content, nil
			},
		)

		return kit.WithTools(t)(a)
	}
}
