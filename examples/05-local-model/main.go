package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/openai"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
)

/*
Example 05 — Local model

The OpenAI provider can target any compatible Responses API backend.
Pass a base URL to point at a local or self-hosted server such as Ollama.

Run:

	go run ./examples/05-local-model

Prerequisites:

	Ollama must be running locally with gemma4 pulled:
	  ollama pull gemma4
*/
func main() {
	ctx := context.Background()

	// Could also be a remote server e.g. "https://your-remote-server.space/v1".
	model, err := openai.New("ollama", "gemma4",
		openai.WithBaseURL("http://localhost:11434/v1"),
	)
	if err != nil {
		log.Fatal(err)
	}

	a, err := agent.New(model,
		agent.WithSystemPrompt("You are a helpful coding assistant with access to the filesystem."),
		agent.WithTools(
			filesystem.NewReadFile(),
			filesystem.NewListDirectory(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	stream, err := a.Stream(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("List the files in the current directory and summarize what this project does.")),
	})
	if err != nil {
		log.Fatal(err)
	}

	for event, err := range stream.Iter() {
		if err != nil {
			log.Fatal(err)
		}

		switch event.Type {
		case kit.EventContentPartStarted:
			switch event.ContentPartType {
			case kit.ContentTypeThinking:
				fmt.Print("[thinking] ")
			case kit.ContentTypeText:
				fmt.Print("[assistant] ")
			}
		case kit.EventContentPartDelta:
			fmt.Print(event.Text)
		case kit.EventContentPartDone:
			if event.ContentPart == nil {
				break
			}

			switch event.ContentPart.Type {
			case kit.ContentTypeThinking, kit.ContentTypeText:
				fmt.Print("\n")
			case kit.ContentTypeToolCall:
				fmt.Printf("[tool %s] requested\n", event.ContentPart.Name)
			case kit.ContentTypeToolResult:
				fmt.Printf("[tool %s] done\n", event.ContentPart.Name)
			}
		}
	}

	_, err = stream.Result()
	if err != nil {
		log.Fatal(err)
	}
}
