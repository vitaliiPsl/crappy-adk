package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/anthropic"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
	"github.com/vitaliiPsl/crappy-adk/providers/openai"
)

/*
Example 04 — Providers

Swapping providers is a one-line change: only the provider and model ID differ.
The agent API is identical regardless of which backend is used.

This example runs the same prompt against Anthropic, Google and OpenAI.
Comment out any provider whose API key you don't have.

Run:

	go run ./examples/04-providers

Prerequisites:

	ANTHROPIC_API_KEY, OPENAI_API_KEY, and GEMINI_API_KEY must be set.
*/
func main() {
	ctx := context.Background()

	prompt := "In one sentence, what is a transformer in machine learning?"

	anthropicModel, err := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"), "claude-haiku-4-5")
	if err != nil {
		log.Fatal(err)
	}

	run(ctx, "anthropic / claude-haiku-4-5", anthropicModel, prompt)

	geminiModel, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")
	if err != nil {
		log.Fatal(err)
	}

	run(ctx, "google / gemini-2.5-flash", geminiModel, prompt)

	openaiModel, err := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-5.4-nano")
	if err != nil {
		log.Fatal(err)
	}

	run(ctx, "openai / gpt-5.4-nano", openaiModel, prompt)
}

func run(ctx context.Context, label string, model kit.Model, prompt string) {
	a, err := agent.New(model, agent.WithSystemPrompt("You are a helpful assistant."))
	if err != nil {
		log.Fatal(err)
	}

	result, err := a.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart(prompt)),
	})
	if err != nil {
		log.Printf("[%s] error: %v", label, err)

		return
	}

	fmt.Printf("[%s]\n%s\n\n", label, result.Output.TextValue())
}
