/*
Example 02 — Agent with tools

Shows how to define a tool using [tool.MustNew] and pass it to an agent.
The agent decides when to call the tool; the loop runs until the model
produces a final text response.

Run:

	go run ./examples/02-tools

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
		func(_ context.Context, args rollArgs) (string, error) {
			if args.Sides < 2 {
				return "", fmt.Errorf("a die needs at least 2 sides")
			}

			return fmt.Sprintf("%d", rand.IntN(args.Sides)+1), nil
		},
	)

	model := openai.New(apiKey, "gpt-5.4-nano")

	a := agent.New(
		model,
		[]kit.Tool{rollDie},
		kit.AgentConfig{
			Instructions: "You are a dramatic dungeon master. Roll a d20 to determine the adventurer's fate, " +
				"then narrate the outcome in two vivid sentences. Never reveal the raw number.",
		},
	)

	messages := []kit.Message{
		kit.NewUserMessage([]kit.Content{
			kit.NewTextContent("I attempt to pick the lock on the dragon's vault."),
		}),
	}

	resp, err := a.Run(context.Background(), messages)
	if err != nil {
		log.Fatalf("agent run failed: %v", err)
	}

	for _, msg := range resp.Messages {
		for _, call := range msg.ToolCalls() {
			fmt.Printf("[tool] %s(%v)\n", call.Name, call.Arguments)
		}

		for _, result := range msg.ToolResults() {
			fmt.Printf("[tool result] %s → %s\n", result.Call.Name, result.Output)
		}
	}

	fmt.Println()
	fmt.Println(resp.Output.Text)
	fmt.Printf("\ntokens — in: %d, out: %d\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
}
