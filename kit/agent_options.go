package kit

// AgentOptions is a functional option for configuring an [Agent].
type AgentOptions func(*Agent)

// WithInstructions sets the system prompt passed to the model on every request.
func WithInstructions(instructions string) AgentOptions {
	return func(a *Agent) {
		a.instructions = instructions
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
