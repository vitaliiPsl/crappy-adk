package anthropic

import "github.com/vitaliiPsl/crappy-adk/kit"

var knownModels = []kit.ModelConfig{
	{
		ID:              "claude-opus-4-6",
		Provider:        providerID,
		ContextWindow:   200_000,
		MaxOutputTokens: 32_000,
		InputPricePerM:  15.00,
		OutputPricePerM: 75.00,
	},
	{
		ID:              "claude-sonnet-4-6",
		Provider:        providerID,
		ContextWindow:   200_000,
		MaxOutputTokens: 64_000,
		InputPricePerM:  3.00,
		OutputPricePerM: 15.00,
	},
	{
		ID:              "claude-haiku-4-5",
		Provider:        providerID,
		ContextWindow:   200_000,
		MaxOutputTokens: 16_000,
		InputPricePerM:  0.80,
		OutputPricePerM: 4.00,
	},
}
