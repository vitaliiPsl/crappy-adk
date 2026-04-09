package compactors

import (
	"context"
	"fmt"
	"strings"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

const summarizeInstruction = `Create a detailed summary of this conversation to use as context 
when continuing. Include these sections:

1. Goal — what the user is trying to accomplish
2. Key concepts — domain terms, constraints, and requirements established
3. Artifacts — any important data, content, decisions, or resources produced or referenced
4. Corrections and decisions — every significant correction or deliberate choice made, what was wrong or considered, what was agreed
5. User messages — every user message verbatim, in order
6. Current state — exactly where the conversation ended and what question or task is next
7. Pending work — concrete remaining tasks

Use bullet points and inline code or content blocks rather than prose. 
Every line should carry information a new agent needs to continue.
Omit greetings, filler, and anything with no future relevance.
This summary must be sufficient to continue without the original conversation.`

// Summarizer compresses conversation history by asking a model to summarize
// older messages while preserving the most recent ones.
type Summarizer struct {
	model       kit.Model
	keepLast    int
	instruction string
}

// Option configures a [Summarizer].
type Option func(*Summarizer)

// WithKeepLast sets how many recent messages to preserve after compaction.
// Defaults to 5.
func WithKeepLast(n int) Option {
	return func(s *Summarizer) { s.keepLast = n }
}

// WithInstruction sets the system prompt used for summarization.
func WithInstruction(instruction string) Option {
	return func(s *Summarizer) { s.instruction = instruction }
}

// NewSummarizer creates a compactor that uses model to summarize conversation
// history. keepLast defaults to 5.
func NewSummarizer(model kit.Model, opts ...Option) *Summarizer {
	s := &Summarizer{
		model:       model,
		keepLast:    5,
		instruction: summarizeInstruction,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

// Compact summarizes older messages while preserving the most recent ones.
func (s *Summarizer) Compact(ctx context.Context, messages []kit.Message) ([]kit.Message, string, error) {
	splitAt := s.splitPoint(messages)
	if splitAt <= 0 {
		return messages, "", nil
	}

	toSummarize := messages[:splitAt]
	toKeep := messages[splitAt:]

	summary, err := s.summarize(ctx, toSummarize)
	if err != nil {
		return nil, "", fmt.Errorf("summarization failed: %w", err)
	}

	summaryMsg := kit.NewSummaryMessage(
		"[Summary of previous conversation]\n\n" + summary +
			"\n\n[End of summary — the conversation continues below]",
	)

	result := make([]kit.Message, 0, 1+len(toKeep))
	result = append(result, summaryMsg)
	result = append(result, toKeep...)

	return result, summary, nil
}

// splitPoint finds the index to split messages at, ensuring we don't break
// apart a tool-call / tool-result group. Returns 0 if there aren't enough
// messages to split.
func (s *Summarizer) splitPoint(messages []kit.Message) int {
	if len(messages) <= s.keepLast+1 {
		return 0
	}

	splitAt := len(messages) - s.keepLast

	// Walk backward to avoid splitting an assistant message from its
	// corresponding tool result messages.
	for splitAt > 0 && messages[splitAt].Role == kit.MessageRoleTool {
		splitAt--
	}

	// If we walked all the way back, there's nothing meaningful to summarize.
	if splitAt <= 0 {
		return 0
	}

	return splitAt
}

func (s *Summarizer) summarize(ctx context.Context, messages []kit.Message) (string, error) {
	formatted := formatMessages(messages)

	resp, err := s.model.Generate(ctx, kit.ModelRequest{
		Instruction: s.instruction,
		Messages:    []kit.Message{kit.NewUserMessage(kit.NewTextPart(formatted))},
	})
	if err != nil {
		return "", err
	}

	return resp.Message.Text(), nil
}

func formatMessages(messages []kit.Message) string {
	var b strings.Builder

	for _, m := range messages {
		switch m.Role {
		case kit.MessageRoleUser:
			fmt.Fprintf(&b, "User: %s", m.Text())
		case kit.MessageRoleAssistant:
			fmt.Fprintf(&b, "Assistant: %s", m.Text())

			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&b, "\n  [Tool call: %s(%v)]", tc.Name, tc.Arguments)
			}
		case kit.MessageRoleTool:
			fmt.Fprintf(&b, "Tool(%s): %s", m.ToolName, m.Text())
		}

		b.WriteString("\n\n")
	}

	return b.String()
}
