package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/google"
	filesystem "github.com/vitaliiPsl/crappy-adk/tools/fs"
	"github.com/vitaliiPsl/crappy-adk/x/middleware"
)

/*
Example 08 — Middleware

Middleware wraps the model and intercepts every Generate and GenerateStream
call. It is transparent to the agent: no changes to Run or Stream.

Use middleware to add cross-cutting behaviour like retry, caching, rate
limiting, or observability at the model layer. Multiple middlewares can be
chained — they apply in the order they are passed to WithModelMiddleware.

This example uses the built-in retry middleware with custom backoff settings.

Run:

	go run ./examples/08-middleware

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
		agent.WithInstruction("You are a helpful coding assistant with access to the filesystem."),
		agent.WithTools(
			filesystem.NewReadFile(),
			filesystem.NewListDirectory(),
		),
		agent.WithModelMiddleware(middleware.NewRetry(
			middleware.WithMaxAttempts(5),
			middleware.WithBaseDelay(300*time.Millisecond),
			middleware.WithMaxDelay(15*time.Second),
		)),
	)
	if err != nil {
		log.Fatal(err)
	}

	result, err := a.Run(ctx, []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("List the files in the current directory and summarize what this project does.")),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Output.Text)
}
