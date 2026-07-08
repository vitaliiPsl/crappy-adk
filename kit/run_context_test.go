package kit

import "testing"

func TestRunContextResponse_OutputComesFromLastModelMessage(t *testing.T) {
	rc := &RunContext{
		Messages: []Message{
			NewUserMessage(NewTextContent("user text")),
			NewModelMessage(NewTextContent("final model")),
		},
	}

	resp := rc.Response()
	if resp.Output == nil {
		t.Fatal("Output is nil")
	}

	if resp.Output.Text != "final model" {
		t.Fatalf("Output = %q, want final model", resp.Output.Text)
	}
}

func TestRunContextResponse_LastNonModelMessageLeavesOutputNil(t *testing.T) {
	rc := &RunContext{
		Messages: []Message{
			NewModelMessage(NewTextContent("older text")),
			NewToolMessage(
				NewToolResultContent(NewToolResult(ToolCall{ID: "call-1", Name: "tool"}, NewToolOutput(NewTextContent("tool output")), nil)),
			),
		},
	}

	resp := rc.Response()
	if resp.Output != nil {
		t.Fatalf("Output = %q, want nil when latest run message is not model", resp.Output.Text)
	}
}
