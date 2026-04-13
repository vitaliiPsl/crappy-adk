package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
	"github.com/vitaliiPsl/crappy-adk/tools/bash"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
)

/*
Example 02 — Streaming

Stream lets you print text as it arrives and observe tool calls in real time.
Events are yielded for lifecycle boundaries such as thinking/content start and done,
for delta chunks while text is streaming, for tool call start/done, and for the
assembled message and tool result objects.

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

	a, err := agent.New(model,
		agent.WithInstruction("You are a helpful coding assistant with access to the filesystem and shell."),
		agent.WithTools(
			bash.NewBash(),
			filesystem.NewReadFile(),
			filesystem.NewListDirectory(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	stream, err := a.Stream(ctx, []kit.Message{
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
		case kit.EventThinkingStarted:
			fmt.Print("[thinking] ")
		case kit.EventThinkingDelta:
			fmt.Print(event.Text)
		case kit.EventThinkingDone:
			fmt.Print("\n")
		case kit.EventContentPartStarted:
			if event.ContentPartType == kit.ContentTypeText {
				fmt.Print("[assistant] ")
			}
		case kit.EventContentPartDelta:
			fmt.Print(event.Text)
		case kit.EventContentPartDone:
			fmt.Print("\n")
		case kit.EventToolCallStarted:
			fmt.Printf("[tool %s] starting\n", event.ToolCall.Name)
		case kit.EventToolCallDone:
			fmt.Printf("[tool %s] requested\n", event.ToolCall.Name)
		case kit.EventToolResult:
			fmt.Printf("[tool %s] done\n", event.ToolResult.Call.Name)
		case kit.EventMessage:
			fmt.Printf("[message %s complete]\n", event.Message.Role)
		}
	}

	result, err := stream.Result()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Printf("final text: %s\n", result.Output.Text)
	fmt.Printf("messages produced: %d\n", len(result.Messages))
}
