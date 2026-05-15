package summarization

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Flatten renders messages as a single text transcript. The flattened form is
// suitable for sending to a summarizer model as one user message, which avoids
// provider-specific constraints on thinking signatures and tool-call continuity
// that apply when the original turn structure is replayed verbatim.
//
// Thinking content is omitted from the transcript.
func Flatten(messages []kit.Message) string {
	var b strings.Builder
	for i, msg := range messages {
		if i > 0 {
			b.WriteByte('\n')
		}

		writeTurn(&b, msg)
	}

	return strings.TrimRight(b.String(), "\n")
}

func writeTurn(b *strings.Builder, msg kit.Message) {
	b.WriteString(roleLabel(msg.Role))
	b.WriteString(":\n")

	for _, c := range msg.Content {
		writeContent(b, c)
	}
}

func writeContent(b *strings.Builder, c kit.Content) {
	switch c.Type {
	case kit.ContentTypeText:
		writeText(b, c.Text)
	case kit.ContentTypeSummary:
		writeSummary(b, c.Summary)
	case kit.ContentTypeToolCall:
		writeToolCall(b, c.ToolCall)
	case kit.ContentTypeToolResult:
		writeToolResult(b, c.ToolResult)
	}
}

func writeText(b *strings.Builder, t *kit.Text) {
	if t == nil {
		return
	}

	b.WriteString(t.Text)
	b.WriteByte('\n')
}

func writeSummary(b *strings.Builder, s *kit.Summary) {
	if s == nil {
		return
	}

	b.WriteString("[Previous summary]\n")
	b.WriteString(s.Text)
	b.WriteByte('\n')
}

func writeToolCall(b *strings.Builder, c *kit.ToolCall) {
	if c == nil {
		return
	}

	args, _ := json.Marshal(c.Arguments)
	fmt.Fprintf(b, "[Tool call: %s(%s)]\n", c.Name, args)
}

func writeToolResult(b *strings.Builder, r *kit.ToolResult) {
	if r == nil {
		return
	}

	if r.Error != "" {
		fmt.Fprintf(b, "[Tool error from %s: %s]\n", r.Call.Name, r.Error)

		return
	}

	fmt.Fprintf(b, "[Tool result from %s: %s]\n", r.Call.Name, r.Output)
}

func roleLabel(r kit.Role) string {
	switch r {
	case kit.RoleUser:
		return "User"
	case kit.RoleModel:
		return "Assistant"
	case kit.RoleTool:
		return "Tool"
	default:
		return string(r)
	}
}
