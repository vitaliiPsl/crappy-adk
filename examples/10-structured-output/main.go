package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"

	"github.com/vitaliiPsl/crappy-adk/providers/google"
)

/*
Example 10 — Structured output

Structured output lets you constrain the model's final answer to a JSON schema.
The provider may enforce the schema natively, and the ADK validates the final
JSON locally before returning it to you.

Run:

	go run ./examples/10-structured-output

Prerequisites:

	GEMINI_API_KEY must be set.
*/
func main() {
	ctx := context.Background()

	model, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")

	if err != nil {
		log.Fatal(err)
	}

	type releaseNotes struct {
		Title      string   `json:"title" jsonschema:"Short release note title"`
		Highlights []string `json:"highlights" jsonschema:"Short highlights for the release note"`
		Breaking   bool     `json:"breaking" jsonschema:"Whether this update is breaking"`
	}

	a, err := agent.New(model,
		agent.WithInstruction("You extract short, factual release notes into JSON. Return only the requested data. Include 2 or 3 highlights."),
		agent.WithResponseSchemaFor[releaseNotes](),
	)
	if err != nil {
		log.Fatal(err)
	}

	result, err := a.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart(
			"Summarize this update as structured release notes: " +
				"'Added structured output support for final answers, " +
				"validated responses locally, and switched Gemini to ResponseJsonSchema.'",
		)),
	})
	if err != nil {
		log.Fatal(err)
	}

	structured := result.StructuredOutput
	if structured == nil {
		log.Fatal("expected structured output, got nil")
	}

	fmt.Println("Validated JSON:")
	fmt.Println(string(structured.JSON))

	var notes releaseNotes
	if err := json.Unmarshal(structured.JSON, &notes); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nTitle: %s\n", notes.Title)
	fmt.Printf("Breaking: %t\n", notes.Breaking)
	fmt.Printf("Highlights: %v\n", notes.Highlights)
}
