package summarization

import (
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// Summarize returns a [Strategy] that summarizes the current memory context and
// records the summary in memory. Memory history is preserved; future context is
// derived from the latest summary message.
func Summarize(model kit.Model, instructions string) Strategy {
	return func(rc *kit.RunContext) error {
		messages, err := rc.Memory.Context(rc.Context)
		if err != nil {
			return fmt.Errorf("summarization: failed to read memory context: %w", err)
		}

		if len(messages) == 0 {
			return nil
		}

		if err := rc.Emit(kit.NewAgentContentStartedEvent(kit.NewSummaryContent(""))); err != nil {
			return err
		}

		resp, err := model.Generate(rc.Context, kit.ModelRequest{
			Instructions: instructions,
			Messages:     messages,
		})
		if err != nil {
			return fmt.Errorf("summarization: summarizer call failed: %w", err)
		}

		text := resp.Message.TextContent()
		if text == nil || text.Text == "" {
			return fmt.Errorf("summarization: summarizer returned empty response")
		}

		rc.Usage.Add(resp.Usage)

		summaryContent := kit.NewSummaryContent(text.Text)
		summary := kit.NewUserMessage(summaryContent)
		rc.Append(summary)

		if err := rc.Memory.Record(rc.Context, summary); err != nil {
			return fmt.Errorf("summarization: failed to record summary: %w", err)
		}

		if err := rc.Emit(kit.NewAgentContentDoneEvent(summaryContent)); err != nil {
			return err
		}

		if err := rc.Emit(kit.NewAgentMessageEvent(summary)); err != nil {
			return err
		}

		return nil
	}
}
