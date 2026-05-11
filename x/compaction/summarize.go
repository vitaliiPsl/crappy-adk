package compaction

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
			return fmt.Errorf("compaction: failed to read memory context: %w", err)
		}

		if len(messages) == 0 {
			return nil
		}

		resp, err := model.Generate(rc.Context, kit.ModelRequest{
			Instructions: instructions,
			Messages:     messages,
		})
		if err != nil {
			return fmt.Errorf("compaction: summarizer call failed: %w", err)
		}

		text := resp.Message.TextContent()
		if text == nil || text.Text == "" {
			return fmt.Errorf("compaction: summarizer returned empty response")
		}

		rc.Usage.Add(resp.Usage)

		summary := kit.NewSummaryMessage(text.Text)
		if err := rc.Memory.Record(rc.Context, summary); err != nil {
			return fmt.Errorf("compaction: failed to record summary: %w", err)
		}

		return nil
	}
}
