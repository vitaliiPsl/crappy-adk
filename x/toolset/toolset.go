// Package toolset provides a static tool registry.
package toolset

import (
	"fmt"
	"slices"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

type Set struct {
	tools  []kit.Tool
	byName map[string]kit.Tool
}

var _ kit.ToolSet = (*Set)(nil)

func New() *Set {
	return &Set{byName: make(map[string]kit.Tool)}
}

// Register rejects duplicates without adding any tools from the batch.
func (s *Set) Register(tools ...kit.Tool) error {
	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		name := tool.Definition().Name
		if _, exists := s.byName[name]; exists || names[name] {
			return fmt.Errorf("duplicate tool %q", name)
		}

		names[name] = true
	}

	for _, tool := range tools {
		s.byName[tool.Definition().Name] = tool
	}

	s.tools = append(s.tools, tools...)

	return nil
}

func (s *Set) Tools() []kit.Tool {
	return slices.Clone(s.tools)
}

func (s *Set) Lookup(name string) (kit.Tool, bool) {
	tool, found := s.byName[name]

	return tool, found
}
