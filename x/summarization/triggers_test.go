package summarization

import (
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestWhenUsageTokensExceed_BelowThreshold(t *testing.T) {
	trigger := WhenUsageTokensExceed(100)

	if trigger(&kit.RunContext{LastUsage: kit.Usage{InputTokens: 50}}) {
		t.Fatal("trigger fired below threshold")
	}
}

func TestWhenUsageTokensExceed_AtThreshold(t *testing.T) {
	trigger := WhenUsageTokensExceed(100)

	if trigger(&kit.RunContext{LastUsage: kit.Usage{InputTokens: 100}}) {
		t.Fatal("trigger fired at exactly threshold")
	}
}

func TestWhenUsageTokensExceed_AboveThreshold(t *testing.T) {
	trigger := WhenUsageTokensExceed(100)

	if !trigger(&kit.RunContext{LastUsage: kit.Usage{InputTokens: 101}}) {
		t.Fatal("trigger did not fire above threshold")
	}
}

func TestWhenUsageTokensExceed_DoesNotFireWhenLastUsageZero(t *testing.T) {
	trigger := WhenUsageTokensExceed(100)

	if trigger(&kit.RunContext{}) {
		t.Fatal("trigger fired without any prior model call")
	}
}

func TestWhenUsageTokensExceed_IgnoresOtherUsageFields(t *testing.T) {
	trigger := WhenUsageTokensExceed(100)

	rc := &kit.RunContext{
		LastUsage: kit.Usage{
			InputTokens:     50,
			OutputTokens:    1_000_000,
			ReasoningTokens: 1_000_000,
			CacheReadTokens: 1_000_000,
		},
	}

	if trigger(rc) {
		t.Fatal("trigger fired based on non-input usage")
	}
}
