package google

import "github.com/vitaliiPsl/crappy-adk/kit"

var knownModels = []kit.ModelConfig{
	{
		ID:              "gemini-3.1-pro-preview",
		Provider:        providerID,
		ContextWindow:   1_048_576,
		MaxOutputTokens: 65_536,
		InputPricePerM:  2.00,
		OutputPricePerM: 12.00,
	},
	{
		ID:              "gemini-3-pro-preview",
		Provider:        providerID,
		ContextWindow:   1_000_000,
		MaxOutputTokens: 64_000,
		InputPricePerM:  2.00,
		OutputPricePerM: 12.00,
	},
	{
		ID:              "gemini-3-flash-preview",
		Provider:        providerID,
		ContextWindow:   1_048_576,
		MaxOutputTokens: 65_536,
		InputPricePerM:  0.50,
		OutputPricePerM: 3.00,
	},
	{
		ID:              "gemini-3.1-flash-lite-preview",
		Provider:        providerID,
		ContextWindow:   1_048_576,
		MaxOutputTokens: 65_536,
		InputPricePerM:  0.50,
		OutputPricePerM: 3.00,
	},
	{
		ID:              "gemini-2.5-pro",
		Provider:        providerID,
		ContextWindow:   1_048_576,
		MaxOutputTokens: 65_536,
		InputPricePerM:  1.25,
		OutputPricePerM: 10.00,
	},
	{
		ID:              "gemini-2.5-flash",
		Provider:        providerID,
		ContextWindow:   1_048_576,
		MaxOutputTokens: 65_536,
		InputPricePerM:  0.30,
		OutputPricePerM: 2.50,
	},
}
