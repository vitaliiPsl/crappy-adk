package subagents

import (
	"context"
	"fmt"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/x/instructions"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	toolName = "agent"
	toolDesc = "Delegate a task to a specialized subagent. Pick the agent from the catalog in your instructions."

	instructionTemplate = `# Subagents

You can delegate tasks to specialized subagents using the agent tool.

Available subagents:
%s`
)

type agentInput struct {
	Agent       string `json:"agent" jsonschema:"Name of the agent to use"`
	Prompt      string `json:"prompt"        jsonschema:"Full task description for the subagent"`
	Description string `json:"description,omitempty"   jsonschema:"Short (3-5 word) label for this task"`
}

func invalid(err error) []agent.Option {
	return []agent.Option{
		func(*agent.Agent) error { return err },
	}
}

func newAgentTool(registry map[string]*agent.Agent, names []string) kit.Tool {
	return tool.MustFunction(
		toolName,
		toolDesc,
		func(ctx context.Context, input agentInput) (string, error) {
			sa, ok := registry[input.Agent]
			if !ok {
				return "", fmt.Errorf("unknown subagent %q, available: %s", input.Agent, strings.Join(names, ", "))
			}

			resp, err := sa.Run(ctx, []kit.Message{
				kit.NewUserMessage(kit.NewTextPart(input.Prompt)),
			})
			if err != nil {
				return "", fmt.Errorf("subagent %q failed: %w", input.Agent, err)
			}

			return resp.Output.Text, nil
		},
	)
}

// WithSubAgents registers a single "agent" tool on the parent that can delegate
// to any of the provided agents. Each agent must have a name and description set
// via [agent.WithName] and [agent.WithDescription].
func WithSubAgents(subagents ...*agent.Agent) []agent.Option {
	if len(subagents) == 0 {
		return []agent.Option{}
	}

	registry := make(map[string]*agent.Agent, len(subagents))

	var catalog strings.Builder

	for _, sa := range subagents {
		if sa == nil {
			return invalid(fmt.Errorf("subagent cannot be nil"))
		}

		if sa.Name() == "" {
			return invalid(fmt.Errorf("subagent must have a name"))
		}

		if _, exists := registry[sa.Name()]; exists {
			return invalid(fmt.Errorf("duplicate subagent name %q", sa.Name()))
		}

		registry[sa.Name()] = sa
		fmt.Fprintf(&catalog, "- %s: %s\n", sa.Name(), sa.Description())
	}

	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}

	instruction := instructions.Static(fmt.Sprintf(instructionTemplate, catalog.String()))

	return []agent.Option{
		agent.WithInstructions(instruction),
		agent.WithTools(newAgentTool(registry, names)),
	}
}
