package agent

import "github.com/vitaliiPsl/crappy-adk/kit"

type toolRegistry struct {
	tools map[string]kit.Tool
	defs  []kit.ToolDefinition
}

func newToolRegistry() *toolRegistry {
	return &toolRegistry{tools: make(map[string]kit.Tool)}
}

func (r *toolRegistry) register(tool kit.Tool) {
	def := tool.Definition()
	r.tools[def.Name] = tool

	for i, existing := range r.defs {
		if existing.Name == def.Name {
			r.defs[i] = def

			return
		}
	}

	r.defs = append(r.defs, def)
}

func (r *toolRegistry) definitions() []kit.ToolDefinition {
	return r.defs
}

func (r *toolRegistry) get(name string) (kit.Tool, bool) {
	tool, ok := r.tools[name]

	return tool, ok
}
