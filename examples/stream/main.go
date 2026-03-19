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

func main() {
	ctx := context.Background()

	provider, err := google.New(ctx, os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	model, err := provider.Model("gemini-2.5-flash")
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
		kit.NewUserMessage(kit.NewTextPart("List the files in the current directory and summarize what this project does.")),
	}

	stream, err := agent.Stream(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	for event, err := range stream.Iter() {
		if err != nil {
			log.Fatal(err)
		}

		switch event.Type {
		case kit.EventTextDelta:
			fmt.Print(event.Text)
		case kit.EventToolCall:
			fmt.Printf("\n[tool call] %s\n", event.ToolCall.Name)
		case kit.EventToolResult:
			fmt.Printf("[tool result] %s\n\n", event.ToolResult.ToolCall.ID)
		}
	}

	fmt.Println()

	// Multi-turn: collect produced messages and continue the conversation.
	messages = append(messages, stream.Result().Messages...)
	messages = append(messages, kit.NewUserMessage(kit.NewTextPart("Which file is the entry point?")))

	stream, err = agent.Stream(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	for event, err := range stream.Iter() {
		if err != nil {
			log.Fatal(err)
		}

		if event.Type == kit.EventTextDelta {
			fmt.Print(event.Text)
		}
	}

	fmt.Println()
}
