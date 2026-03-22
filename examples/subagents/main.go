package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kit/tool"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
)

// This example shows a parent orchestrator agent that delegates tasks to two
// specialized subagents:
//
//   - researcher: explores the codebase and gathers information
//   - writer:     produces a structured summary from provided content
//
// The orchestrator decides which subagent to call and in what order based on
// the user's request. It uses the "delegate" tool registered by WithSubAgents.

func main() {
	ctx := context.Background()

	provider := google.New()

	model, err := provider.Model(ctx, "gemini-2.5-flash", os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	// Researcher: has filesystem access, no writing capability.
	researcher, err := kit.NewAgent(model,
		kit.WithInstruction(`You are a code researcher.
Explore the codebase using the tools available to you and answer questions with detailed, factual findings.
Always cite specific files and line numbers when relevant.`),
		kit.WithTools(
			filesystem.NewListDirectory(),
			filesystem.NewReadFile(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Writer: no filesystem access, focuses on structuring content.
	writer, err := kit.NewAgent(model,
		kit.WithInstruction(`You are a technical writer.
Given raw findings or notes, produce clear and well-structured documentation.
Use markdown with headers, bullet points, and code blocks where appropriate.`),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Orchestrator: delegates to researcher and writer via the delegate tool.
	orchestrator, err := kit.NewAgent(model,
		kit.WithInstruction(`You are an orchestrator. Use the delegate tool to assign tasks to the appropriate subagent.`),
		tool.WithSubAgents(
			tool.SubAgent{
				Name:        "researcher",
				Description: "Explores the codebase and answers factual questions about code structure, types, and logic.",
				Agent:       researcher,
			},
			tool.SubAgent{
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
		case kit.EventTextDelta:
			fmt.Print(event.Text)
		case kit.EventToolCall:
			fmt.Printf("\n[delegate → %s]\n", event.ToolCall.Arguments["agent"])
		case kit.EventToolResult:
			fmt.Printf("[delegate ← done]\n\n")
		}
	}

	fmt.Println()
}
