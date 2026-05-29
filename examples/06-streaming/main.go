/*
Example 06 — Streaming with thinking and tools

Shows how to consume an agent stream, handling thinking tokens, text tokens,
tool calls, and tool results as they arrive.

Run:

	go run ./examples/06-streaming

Prerequisites:

	OPENAI_API_KEY must be set.
*/
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/openai"
	"github.com/vitaliiPsl/crappy-adk/x/memory"
	tool "github.com/vitaliiPsl/crappy-adk/x/tool"
)

type rollArgs struct {
	Sides int `json:"sides" jsonschema:"Number of sides on the die, e.g. 6 or 20"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is not set")
	}

	rollDie := tool.MustNew(
		"roll_die",
		"Roll a die with the given number of sides and return the result.",
		func(_ *kit.RunContext, args rollArgs) (string, error) {
			if args.Sides < 2 {
				return "", fmt.Errorf("a die needs at least 2 sides")
			}

			return fmt.Sprintf("%d", rand.IntN(args.Sides)+1), nil
		},
	)

	model, err := openai.New(apiKey, "gpt-5.4-nano")
	if err != nil {
		log.Fatalf("failed to create model: %v", err)
	}

	a, err := agent.New(
		model,
		memory.NewHistory(),
		agent.WithInstructions(
			"You are a dramatic dungeon master. Roll a d20 to determine the adventurer's fate, "+
				"then narrate the outcome in two vivid sentences. Never reveal the raw number.",
		),
		agent.WithThinking(kit.ThinkingLevelLow),
		agent.WithTools(rollDie),
	)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	input := kit.NewUserMessage(
		kit.NewTextContent("I attempt to pick the lock on the dragon's vault."))

	stream := a.Stream(context.Background(), input)

	for event := range stream.Iter() {
		switch {
		case event.Content != nil:
			handleContentEvent(event)
		case event.ToolResult != nil:
			fmt.Printf("[tool result] %s → %s\n\n", event.ToolResult.Call.Name, event.ToolResult.Output)
		}
	}

	resp, err := stream.Result()
	if err != nil {
		log.Fatalf("stream failed: %v", err)
	}

	fmt.Printf("tokens — in: %d, out: %d (reasoning: %d)\n",
		resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.ReasoningTokens)
}

func handleContentEvent(event kit.AgentEvent) {
	switch event.Type {
	case kit.EventContentStarted:
		switch event.Content.Type {
		case kit.ContentTypeThinking:
			fmt.Println("[thinking]")
		case kit.ContentTypeText:
			fmt.Println("[response]")
		case kit.ContentTypeToolCall:
			fmt.Printf("[tool call] %s\n", event.Content.ToolCall.Name)
		}
	case kit.EventContentDelta:
		switch event.Content.Type {
		case kit.ContentTypeThinking:
			fmt.Print(event.Content.Thinking.Text)
		case kit.ContentTypeText:
			fmt.Print(event.Content.Text.Text)
		}
	case kit.EventContentDone:
		switch event.Content.Type {
		case kit.ContentTypeThinking, kit.ContentTypeText:
			fmt.Println()
		}
	}
}
