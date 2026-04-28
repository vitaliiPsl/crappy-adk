package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
	"github.com/vitaliiPsl/crappy-adk/x/limits"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

/*
Example 09 — Limits

Limit guards constrain how much work one agent run can do. They use the existing
hook system, so the core agent loop stays unchanged.

This example intentionally sets a low turn limit. The model can call one tool,
then the limit guard stops the run before a second model turn starts.

Run:

	go run ./examples/09-limits

Prerequisites:

	GEMINI_API_KEY must be set.
*/
func main() {
	ctx := context.Background()

	model, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")
	if err != nil {
		log.Fatal(err)
	}

	type lookupInput struct {
		Query string `json:"query" jsonschema:"Short lookup query"`
	}

	lookup := tool.MustFunction(
		"lookup",
		"Look up a tiny fact from the local demo dataset.",
		func(_ context.Context, input lookupInput) (string, error) {
			return fmt.Sprintf("demo result for %q: limit guards stop runaway agent work", input.Query), nil
		},
	)

	a, err := agent.New(model,
		agent.WithSystemPrompt("Use the lookup tool first, then explain the result in a final answer."),
		agent.WithTools(lookup),
		limits.WithMaxTurns(1),
		limits.WithMaxToolCalls(3),
		limits.WithToolLoopDetection(2, 15),
	)
	if err != nil {
		log.Fatal(err)
	}

	result, err := a.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Look up what limit guards do.")),
	})
	if err != nil {
		if errors.Is(err, kit.ErrLimitExceeded) {
			fmt.Printf("Limits stopped the run after %d generated message(s): %v\n", len(result.Messages), err)

			return
		}

		if errors.Is(err, kit.ErrToolLoop) {
			fmt.Printf("Tool loop stopped the run after %d generated message(s): %v\n", len(result.Messages), err)

			return
		}

		log.Fatal(err)
	}

	fmt.Println(result.Output.Text)
}
