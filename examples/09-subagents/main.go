package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/extensions/subagents"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
	"github.com/vitaliiPsl/crappy-adk/x/instructions"
)

/*
Example 09 — Subagents

WithSubAgents registers a delegate tool on the parent agent. When called,
it runs the target subagent's full ReAct loop and returns its output.

Each subagent has its own instruction set and tool access. The orchestrator
decides which subagent to call and in what order based on the task.

Run:

	go run ./examples/09-subagents

Prerequisites:

	GEMINI_API_KEY must be set.
*/
func main() {
	ctx := context.Background()

	model, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")
	if err != nil {
		log.Fatal(err)
	}

	workdir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	researcher, err := agent.New(model,
		agent.WithInstruction(`You are a code researcher.
Explore the codebase using the tools available and answer questions with detailed, factual findings.
Always cite specific files and line numbers when relevant.`),
		agent.WithInstructions(instructions.Env(workdir)),
		agent.WithTools(
			filesystem.NewListDirectory(),
			filesystem.NewReadFile(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	writer, err := agent.New(model,
		agent.WithInstruction(`You are a technical writer.
Given raw findings or notes, produce clear and well-structured documentation.
Use markdown with headers, bullet points, and code blocks where appropriate.`),
	)
	if err != nil {
		log.Fatal(err)
	}

	orchestrator, err := agent.New(model,
		agent.WithInstruction(`You are an orchestrator. You must always delegate — never answer directly.
Always follow this sequence: first delegate research tasks to the researcher, then pass the findings to the writer to produce the final output.`),
		subagents.WithSubAgents(
			subagents.SubAgent{
				Name:        "researcher",
				Description: "Explores the codebase and answers factual questions about code structure, types, and logic.",
				Agent:       researcher,
			},
			subagents.SubAgent{
				Name:        "writer",
				Description: "Takes raw findings or notes and turns them into well-structured markdown documentation.",
				Agent:       writer,
			},
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	stream, err := orchestrator.Stream(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("Produce a short developer overview of this project: what it does, its main packages, and key abstractions.")),
	})
	if err != nil {
		log.Fatal(err)
	}

	for event, err := range stream.Iter() {
		if err != nil {
			log.Fatal(err)
		}

		switch event.Type {
		case kit.EventContentPartDelta:
			fmt.Print(event.Text)
		case kit.EventToolCallDone:
			fmt.Printf("\n[delegate → %s]\n", event.ToolCall.Arguments["agent"])
		case kit.EventToolResult:
			fmt.Printf("[delegate ← done]\n\n")
		}
	}

	fmt.Println()
}
