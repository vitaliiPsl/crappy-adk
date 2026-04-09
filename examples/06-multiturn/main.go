package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
)

/*
Example 06 — Multi-turn conversation

The agent is stateless between runs. To continue a conversation, append the
messages produced by the previous run before calling Run again.

Run:

	go run ./examples/06-multiturn

Prerequisites:

	GEMINI_API_KEY must be set.
*/

// TODO: maybe i should actually consider stateful agents.
// Current agent implementation could become some kind of flow/runner and agent would be more of a wrapper with state.
func main() {
	ctx := context.Background()

	provider := google.New()

	model, err := provider.Model(ctx, "gemini-2.5-flash", os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	agent, err := kit.NewAgent(model,
		kit.WithInstruction("You are a helpful coding assistant with access to the filesystem."),
		kit.WithTools(
			filesystem.NewReadFile(),
			filesystem.NewListDirectory(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	turns := []string{
		"List the files in the current directory and summarize what this project does.",
		"Which dependencies does it have?",
	}

	var messages []kit.Message

	for _, prompt := range turns {
		messages = append(messages, kit.NewUserMessage(kit.NewTextPart(prompt)))

		fmt.Printf("User: %s\n\n", prompt)

		result, err := agent.Run(ctx, messages)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Agent: %s\n\n", result.Output.Text)

		// Carry forward everything produced in this run before the next turn.
		messages = append(messages, result.Messages...)
	}
}
