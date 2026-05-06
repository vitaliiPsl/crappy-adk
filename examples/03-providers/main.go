/*
Example 03 — Providers

Runs the same prompt through Anthropic, Google, and OpenAI to show that
the agent is provider-agnostic.

Run:

	go run ./examples/03-providers

Prerequisites:

	ANTHROPIC_API_KEY, GEMINI_API_KEY, and OPENAI_API_KEY must be set.
*/
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

func main() {
	ctx := context.Background()

	prompt := "In one sentence, what is a transformer in machine learning?"

	anthropicModel := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"), "claude-haiku-4-5")
	run(ctx, "anthropic / claude-haiku-4-5", anthropicModel, prompt)

	geminiModel, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-3-flash-preview")
	if err != nil {
		log.Fatal(err)
	}

	run(ctx, "google / gemini-3-flash-preview", geminiModel, prompt)

	openaiModel := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-5.4-nano")
	run(ctx, "openai / gpt-5.4-nano", openaiModel, prompt)
}

func run(ctx context.Context, label string, model kit.Model, prompt string) {
	a, err := agent.New(model, agent.WithInstructions("You are a helpful assistant."))
	if err != nil {
		log.Printf("[%s] failed to create agent: %v", label, err)

		return
	}

	resp, err := a.Run(ctx, []kit.Message{
		kit.NewUserMessage([]kit.Content{kit.NewTextContent(prompt)}),
	})
	if err != nil {
		log.Printf("[%s] error: %v", label, err)

		return
	}

	fmt.Printf("[%s]\n%s\n\n", label, resp.Output.Text)
}
