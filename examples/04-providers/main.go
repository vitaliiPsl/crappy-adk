package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	anthropicModel, err := anthropic.New().Model(ctx, "claude-haiku-4-5", os.Getenv("ANTHROPIC_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	run(ctx, "anthropic / claude-haiku-4-5", anthropicModel, prompt)

	geminiModel, err := google.New().Model(ctx, "gemini-2.5-flash", os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	run(ctx, "google / gemini-2.5-flash", geminiModel, prompt)

	openaiModel, err := openai.New().Model(ctx, "gpt-5.4-nano", os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	run(ctx, "openai / gpt-5.4-nano", openaiModel, prompt)
}

func run(ctx context.Context, label string, model kit.Model, prompt string) {
	agent, err := kit.NewAgent(model, kit.WithInstruction("You are a helpful assistant."))
	if err != nil {
		log.Fatal(err)
	}

	result, err := agent.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart(prompt)),
	})
	if err != nil {
		log.Printf("[%s] error: %v", label, err)
		return
	}

	fmt.Printf("[%s]\n%s\n\n", label, result.Output.Text)
}
