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

	agent := kit.NewAgent(model,
		kit.WithInstructions("You are a helpful coding assistant with access to the filesystem."),
		kit.WithTools(
			filesystem.NewReadFile(),
			filesystem.NewEditFile(),
			filesystem.NewWriteFile(),
			filesystem.NewListDirectory(),
		),
	)

	reply, err := agent.Run(ctx, []kit.Message{
		kit.NewUserMessage("List the files in the current directory and summarize what this project does."),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(reply.Content)
}
