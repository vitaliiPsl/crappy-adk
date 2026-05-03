/*
Example 01 — Basic agent

Minimal setup: one instruction, no tools, one Run call.

Run:

	go run ./examples/01-basic

Prerequisites:

	OPENAI_API_KEY must be set.
*/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/openai"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is not set")
	}

	model := openai.New(apiKey, "gpt-4.1-mini")

	a := agent.New(
		"You are a philosopher who has seen too much and suffers fools reluctantly. "+
			"Answer every question correctly, but make it clear the question has wounded you personally.",
		model,
		nil,
	)

	messages := []kit.Message{
		kit.NewUserMessage([]kit.Content{
			kit.NewTextContent("If a tree falls in the forest and nobody is around to hear it, does it make a sound?"),
		}),
	}

	resp, err := a.Run(context.Background(), messages)
	if err != nil {
		log.Fatalf("agent run failed: %v", err)
	}

	fmt.Println(resp.Output.Text)
	fmt.Printf("\ntokens — in: %d, out: %d\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
}
