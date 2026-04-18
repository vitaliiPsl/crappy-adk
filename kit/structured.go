package kit

// StructuredOutput is a validated machine-readable final answer.
type StructuredOutput struct {
	// JSON is the normalized JSON encoding of the validated output.
	JSON []byte `json:"json"`
}
