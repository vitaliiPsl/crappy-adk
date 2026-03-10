package kit

type AgentOptions func(*Agent)

func WithInstructions(instructions string) AgentOptions {
	return func(a *Agent) {
		a.instructions = instructions
	}
}

func WithTool(tool Tool) AgentOptions {
	return func(a *Agent) {
		a.tools[tool.Definition().Name] = tool
	}
}

func WithTools(tools ...Tool) AgentOptions {
	return func(a *Agent) {
		for _, tool := range tools {
			a.tools[tool.Definition().Name] = tool
		}
	}
}
