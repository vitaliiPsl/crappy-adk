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

	messages := []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("List the files in the current directory and summarize what this project does.")),
	}

	resp, err := agent.Run(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.LastMessage().Text())

	// Append new messages produced this run, then ask a follow-up.
	messages = append(messages, resp.Messages...)
	messages = append(messages, kit.NewUserMessage(kit.NewTextPart("Which file is the entry point?")))

	resp, err = agent.Run(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.LastMessage().Text())
}
