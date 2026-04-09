package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
	"github.com/vitaliiPsl/crappy-adk/tools/bash"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
)

/*
Example 02 — Streaming

Stream lets you print text as it arrives and observe tool calls in real time.
Events are yielded per token (text_delta), per thinking token (thinking_delta)
for models that support extended thinking, per tool invocation (tool_call),
and per tool result (tool_result).

Run:

	go run ./examples/02-stream

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
		kit.WithInstruction("You are a helpful coding assistant with access to the filesystem and shell."),
		kit.WithTools(
			bash.NewBash(),
			filesystem.NewReadFile(),
			filesystem.NewListDirectory(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	stream, err := agent.Stream(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("List the files in the current directory and tell me what this project does.")),
	})
	if err != nil {
		log.Fatal(err)
	}

	for event, err := range stream.Iter() {
		if err != nil {
			log.Fatal(err)
		}

		switch event.Type {
		case kit.EventThinkingDelta:
			fmt.Print(event.Text)
		case kit.EventTextDelta:
			fmt.Print(event.Text)
		case kit.EventToolCall:
			fmt.Printf("\n[%s]\n", event.ToolCall.Name)
		case kit.EventToolResult:
			fmt.Printf("[%s done]\n\n", event.ToolResult.Call.Name)
		}
	}

	fmt.Println()
}
