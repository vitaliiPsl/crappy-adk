package openai

import "github.com/vitaliiPsl/crappy-adk/kit"

var knownModels = []kit.ModelConfig{
	{
		ID:              "gpt-5.4",
		Provider:        providerID,
		ContextWindow:   1_050_000,
		MaxOutputTokens: 128_000,
		InputPricePerM:  2.50,
		OutputPricePerM: 15.00,
	},
	{
		ID:              "gpt-5.4-pro",
		Provider:        providerID,
		ContextWindow:   1_050_000,
		MaxOutputTokens: 128_000,
		InputPricePerM:  30.00,
		OutputPricePerM: 180.00,
	},
	{
		ID:              "gpt-5-mini",
		Provider:        providerID,
		ContextWindow:   400_000,
		MaxOutputTokens: 128_000,
		InputPricePerM:  0.25,
		OutputPricePerM: 2.00,
	},
	{
		ID:              "gpt-5-nano",
		Provider:        providerID,
		ContextWindow:   400_000,
		MaxOutputTokens: 128_000,
		InputPricePerM:  0.05,
		OutputPricePerM: 0.40,
	},
	{
		ID:              "gpt-5",
		Provider:        providerID,
		ContextWindow:   400_000,
		MaxOutputTokens: 128_000,
		InputPricePerM:  1.25,
		OutputPricePerM: 10.00,
	},
	{
		ID:              "gpt-4.1",
		Provider:        providerID,
		ContextWindow:   1_047_576,
		MaxOutputTokens: 32_768,
		InputPricePerM:  2.00,
		OutputPricePerM: 8.00,
	},
}
