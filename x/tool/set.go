package tool

import "github.com/vitaliiPsl/crappy-adk/kit"

var _ kit.ToolSet = (*Set)(nil)

// Set is an ordered, name-indexed [kit.ToolSet]. The first registration of a
// given name wins; later tools with the same name are ignored.
type Set struct {
	order []kit.Tool
	index map[string]kit.Tool
}

// NewSet creates a [Set] containing the given tools.
func NewSet(tools ...kit.Tool) *Set {
	set := &Set{index: make(map[string]kit.Tool)}
	set.Add(tools...)

	return set
}

func (s *Set) Add(tools ...kit.Tool) {
	for _, tool := range tools {
		name := tool.Definition().Name
		if _, exists := s.index[name]; exists {
			continue
		}

		s.index[name] = tool
		s.order = append(s.order, tool)
	}
}

func (s *Set) Get(name string) (kit.Tool, bool) {
	tool, ok := s.index[name]

	return tool, ok
}

func (s *Set) List() []kit.Tool {
	return s.order
}
