package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/custom"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
)

/*
Example 05 — Local model

The custom provider wraps any OpenAI-compatible inference server.
Pass a base URL to target a specific server, or leave it empty to
default to Ollama at localhost:11434.

Run:

	go run ./examples/05-local-model

Prerequisites:

	Ollama must be running locally with gemma4 pulled:
	  ollama pull gemma4
*/
func main() {
	ctx := context.Background()

	// Could also be a remote server e.g. "https://your-remote-server.space/v1".
	model, err := custom.New("http://localhost:11434/v1").Model(ctx, "gemma4", "")
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

	result, err := agent.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("List the files in the current directory and summarize what this project does.")),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Output.Text)
}
