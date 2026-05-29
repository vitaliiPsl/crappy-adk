package kit

// ToolSet is a collection of tools an agent can use, indexed by name and kept
// in registration order.
type ToolSet interface {
	// Add registers tools, keeping the first registration of each name.
	Add(tools ...Tool)
	// Get returns the tool registered under name.
	Get(name string) (Tool, bool)
	// List returns the tools in registration order.
	List() []Tool
}
