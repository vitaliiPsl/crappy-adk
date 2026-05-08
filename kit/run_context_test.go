package kit

import "testing"

func TestRunContextResponse_OutputComesFromLastModelMessage(t *testing.T) {
	rc := &RunContext{
		Generated: []Message{
			NewUserMessage([]Content{NewTextContent("user text")}),
			NewModelMessage([]Content{NewTextContent("final model")}),
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
		Generated: []Message{
			NewModelMessage([]Content{NewTextContent("older text")}),
			NewToolMessage([]Content{
				NewToolResultContent(NewToolResult(ToolCall{ID: "call-1", Name: "tool"}, "tool output", nil)),
			}),
		},
	}

	resp := rc.Response()
	if resp.Output != nil {
		t.Fatalf("Output = %q, want nil when latest generated message is not model", resp.Output.Text)
	}
}
