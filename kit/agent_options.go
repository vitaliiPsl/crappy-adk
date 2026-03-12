package kit

import "context"

// AgentOptions is a functional option for configuring an [Agent].
type AgentOptions func(*Agent)

// WithInstruction adds a static string to the agent's system prompt.
func WithInstruction(text string) AgentOptions {
	return WithInstructions(func(_ context.Context) (string, error) {
		return text, nil
	})
}

// WithInstructions appends one or more [Instruction] values to the
// agent's system prompt. Sources are evaluated in order on each [Agent.Run].
func WithInstructions(sources ...Instruction) AgentOptions {
	return func(a *Agent) {
		a.instructions = append(a.instructions, sources...)
	}
}

// WithTool registers a single tool with the agent.
func WithTool(tool Tool) AgentOptions {
	return func(a *Agent) {
		a.tools[tool.Definition().Name] = tool
	}
}

// WithTools registers multiple tools with the agent.
func WithTools(tools ...Tool) AgentOptions {
	return func(a *Agent) {
		for _, tool := range tools {
			a.tools[tool.Definition().Name] = tool
		}
	}
}

// WithGenerationConfig sets the generation parameters used on every model request.
func WithGenerationConfig(config GenerationConfig) AgentOptions {
	return func(a *Agent) {
		a.generationConfig = config
	}
}
