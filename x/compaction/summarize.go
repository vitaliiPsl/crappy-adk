package compaction

import (
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// SummarizeWithRecent returns a [Strategy] that summarizes all but the last
// keepRecent messages, replacing the summarized prefix with a single summary
// message and appending the summary to [kit.RunContext.Generated].
func SummarizeWithRecent(model kit.Model, instructions string, keepRecent int) Strategy {
	return func(rc *kit.RunContext) error {
		cutoff := safeCutoff(rc.Messages, keepRecent)
		if cutoff <= 0 {
			return nil
		}

		toSummarize := rc.Messages[:cutoff]

		resp, err := model.Generate(rc.Context, kit.ModelRequest{
			Instructions: instructions,
			Messages:     toSummarize,
		})
		if err != nil {
			return fmt.Errorf("compaction: summarizer call failed: %w", err)
		}

		text := resp.Message.TextContent()
		if text == nil || text.Text == "" {
			return fmt.Errorf("compaction: summarizer returned empty response")
		}

		rc.Usage.Add(resp.Usage)

		summary := kit.NewUserMessage([]kit.Content{kit.NewSummaryContent(text.Text)})

		rc.Messages = append([]kit.Message{summary}, rc.Messages[cutoff:]...)
		rc.Generated = append(rc.Generated, summary)

		return nil
	}
}

func safeCutoff(msgs []kit.Message, keepRecent int) int {
	desired := len(msgs) - keepRecent
	if desired <= 0 {
		return 0
	}

	for desired < len(msgs) && msgs[desired].Role == kit.RoleTool {
		desired++
	}

	return desired
}
