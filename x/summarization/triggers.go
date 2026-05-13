package summarization

import "github.com/vitaliiPsl/crappy-adk/kit"

// WhenUsageTokensExceed fires when the input tokens reported by the most
// recent model response exceed threshold.
func WhenUsageTokensExceed(threshold int64) Trigger {
	return func(rc *kit.RunContext) bool {
		return rc.LastUsage.InputTokens > threshold
	}
}
