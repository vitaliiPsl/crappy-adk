package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
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
		kit.WithInstructions("You are a helpful assistant."),
	)

	reply, err := agent.Run(ctx, []kit.Message{
		kit.NewUserMessage("What is the capital of France?"),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(reply.Content)
}
