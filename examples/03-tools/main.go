package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kit/tool"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
)

/*
Example 03 — Custom tools with FunctionTool

FunctionTool[T] wraps a typed Go function as a tool. The JSON schema for
the arguments is generated automatically from the exec function signature.

Run:

	go run ./examples/03-tools

Prerequisites:

	GEMINI_API_KEY must be set.
*/
func main() {
	ctx := context.Background()

	model, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")
	if err != nil {
		log.Fatal(err)
	}

	type GetTimeInput struct {
		Timezone string `json:"timezone" jsonschema:"IANA timezone name, e.g. America/New_York"`
	}

	getTime, err := tool.NewFunction(
		"get_time",
		"Get the current time in a given IANA timezone.",
		func(_ context.Context, args GetTimeInput) (string, error) {
			loc, err := time.LoadLocation(args.Timezone)
			if err != nil {
				return "", fmt.Errorf("unknown timezone: %s", args.Timezone)
			}

			return time.Now().In(loc).Format(time.RFC3339), nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	agent, err := kit.NewAgent(model,
		kit.WithInstruction("You are a helpful assistant. Use tools when needed."),
		kit.WithTools(getTime),
	)
	if err != nil {
		log.Fatal(err)
	}

	result, err := agent.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("What time is it right now in Tokyo, London, and New York?")),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Output.Text)
}
