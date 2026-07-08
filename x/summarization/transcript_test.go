package summarization

import (
	"strings"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestFlatten_Empty(t *testing.T) {
	if got := Flatten(nil); got != "" {
		t.Fatalf("Flatten(nil) = %q, want empty", got)
	}
}

func TestFlatten_LabelsRolesAndJoinsTurns(t *testing.T) {
	msgs := []kit.Message{
		kit.NewUserMessage(kit.NewTextContent("hello")),
		kit.NewModelMessage(kit.NewTextContent("hi there")),
	}

	got := Flatten(msgs)

	if !strings.Contains(got, "User:\nhello") {
		t.Errorf("missing user turn:\n%s", got)
	}

	if !strings.Contains(got, "Assistant:\nhi there") {
		t.Errorf("missing assistant turn:\n%s", got)
	}
}

func TestFlatten_SkipsThinking(t *testing.T) {
	msgs := []kit.Message{
		kit.NewModelMessage(
			kit.NewThinkingContent("id", "secret reasoning", "sig"),
			kit.NewTextContent("public answer"),
		),
	}

	got := Flatten(msgs)

	if strings.Contains(got, "secret reasoning") {
		t.Errorf("thinking leaked into transcript:\n%s", got)
	}

	if strings.Contains(got, "sig") {
		t.Errorf("thinking signature leaked into transcript:\n%s", got)
	}

	if !strings.Contains(got, "public answer") {
		t.Errorf("missing text content:\n%s", got)
	}
}

func TestFlatten_RendersSummary(t *testing.T) {
	msgs := []kit.Message{
		kit.NewUserMessage(kit.NewSummaryContent("earlier conversation recap")),
		kit.NewUserMessage(kit.NewTextContent("next question")),
	}

	got := Flatten(msgs)

	if !strings.Contains(got, "[Previous summary]\nearlier conversation recap") {
		t.Errorf("missing summary block:\n%s", got)
	}

	if !strings.Contains(got, "next question") {
		t.Errorf("missing user text:\n%s", got)
	}
}

func TestFlatten_RendersToolCallAndResult(t *testing.T) {
	call := kit.NewToolCall("call-1", "bash", map[string]any{"cmd": "ls"})
	result := kit.NewToolResult(call, kit.NewToolOutput(kit.NewTextContent("file.txt\n")), nil)

	msgs := []kit.Message{
		kit.NewModelMessage(kit.NewToolCallContent(call)),
		kit.NewToolMessage(kit.NewToolResultContent(result)),
	}

	got := Flatten(msgs)

	if !strings.Contains(got, `[Tool call: bash({"cmd":"ls"})]`) {
		t.Errorf("missing tool call:\n%s", got)
	}

	if !strings.Contains(got, "[Tool result from bash]\nfile.txt") {
		t.Errorf("missing tool result:\n%s", got)
	}
}

func TestFlatten_RendersToolError(t *testing.T) {
	call := kit.NewToolCall("call-1", "bash", map[string]any{"cmd": "boom"})
	result := kit.NewToolResult(call, kit.ToolOutput{}, errString("permission denied"))

	msgs := []kit.Message{
		kit.NewToolMessage(kit.NewToolResultContent(result)),
	}

	got := Flatten(msgs)

	if !strings.Contains(got, "[Tool error from bash: permission denied]") {
		t.Errorf("missing tool error:\n%s", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
