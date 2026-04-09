package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
)

/*
Example 01 — Basic agent

Minimal setup: one instruction, no tools, one Run call.

Run:

	go run ./examples/01-basic

Prerequisites:

	GEMINI_API_KEY must be set.
*/
func main() {
	ctx := context.Background()

	model, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")
	if err != nil {
		log.Fatal(err)
	}

	agent, err := kit.NewAgent(model,
		kit.WithInstruction("You are a concise assistant. Answer in one or two sentences."),
	)
	if err != nil {
		log.Fatal(err)
	}

	result, err := agent.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("What is the ReAct pattern in LLM agents?")),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Output.Text)
}
