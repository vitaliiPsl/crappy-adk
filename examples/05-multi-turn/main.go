/*
Example 05 — Multi-turn conversation

Shows how to carry conversation history across multiple [agent.Agent.Run] calls.
After each turn, the agent's response messages are appended to the shared history,
so the next turn has full context of what was said before.

Run:

	go run ./examples/05-multi-turn

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
	"github.com/vitaliiPsl/crappy-adk/x/memory"
	tool "github.com/vitaliiPsl/crappy-adk/x/tool"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is not set")
	}

	model, err := openai.New(apiKey, "gpt-5.4-nano")
	if err != nil {
		log.Fatalf("failed to create model: %v", err)
	}

	a, err := agent.New(
		model,
		memory.NewHistory(),
		tool.NewSet(),
		agent.WithInstructions("You are a concise assistant. Keep every reply to two sentences or fewer."),
	)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	turns := []string{
		"My name is Vitalii and I am working on this AI assistant called Crappy.",
		"What is my name?",
		"What am I working on?",
	}

	var totalUsage kit.Usage

	ctx := context.Background()
	for i, prompt := range turns {
		input := kit.NewUserMessage(
			kit.NewTextContent(prompt))

		resp, err := a.Run(ctx, input)
		if err != nil {
			log.Fatalf("turn %d failed: %v", i+1, err)
		}

		totalUsage.Add(resp.Usage)

		fmt.Printf("User: %s\n", prompt)
		fmt.Printf("Agent: %s\n\n", resp.Output.Text)
	}

	fmt.Printf("total tokens — in: %d, out: %d\n", totalUsage.InputTokens, totalUsage.OutputTokens)
}
