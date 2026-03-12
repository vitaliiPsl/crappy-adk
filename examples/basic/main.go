package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/anthropic"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
)

func main() {
	ctx := context.Background()

	provider := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))

	model, err := provider.Model("claude-haiku-4-5")
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

	messages := []kit.Message{
		kit.NewUserMessage("List the files in the current directory and summarize what this project does."),
	}

	// UI — render text in real time
	for event, err := range agent.Run(ctx, messages) {
		if err != nil {
			log.Fatal(err)
		}
		if event.Delta != nil {
			fmt.Print(event.Delta.Text)
		}
	}
	fmt.Println()

	// Both — stream text while building history for multi-turn conversations
	var history []kit.Message
	history = append(history, messages...)

	for event, err := range agent.Run(ctx, history) {
		if err != nil {
			log.Fatal(err)
		}
		switch {
		case event.Delta != nil:
			fmt.Print(event.Delta.Text)
		case event.Message != nil:
			history = append(history, *event.Message)
		}
	}
	fmt.Println()

	// Follow-up — pass history with a new user message to continue the conversation
	history = append(history, kit.NewUserMessage("Which file is the entry point?"))

	for event, err := range agent.Run(ctx, history) {
		if err != nil {
			log.Fatal(err)
		}
		switch {
		case event.Delta != nil:
			fmt.Print(event.Delta.Text)
		case event.Message != nil:
			history = append(history, *event.Message)
		}
	}
	fmt.Println()
}
