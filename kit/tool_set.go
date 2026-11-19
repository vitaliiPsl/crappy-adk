package kit

// ToolSet may expose fewer tools than it can resolve. Reads must be side-effect free.
type ToolSet interface {
	Register(tools ...Tool) error
	Tools() []Tool
	Lookup(name string) (Tool, bool)
}
