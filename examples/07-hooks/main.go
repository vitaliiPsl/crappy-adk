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
)

/*
Example 07 — Hooks

Hooks let you intercept every stage of the ReAct loop without modifying
agent logic. Useful for logging, tracing, metrics, tool permissions, and
request mutation.

This example logs token usage per turn and measures tool execution time.

Run:

	go run ./examples/07-hooks

Prerequisites:

	GEMINI_API_KEY must be set.
*/
func main() {
	ctx := context.Background()

	model, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")
	if err != nil {
		log.Fatal(err)
	}

	toolStartTimes := map[string]time.Time{}

	a, err := agent.New(model,
		agent.WithSystemPrompt("You are a helpful coding assistant with access to the filesystem."),
		agent.WithTools(
			filesystem.NewReadFile(),
			filesystem.NewListDirectory(),
		),
		agent.WithOnModelResponse(func(ctx context.Context, resp kit.ModelResponse) (context.Context, kit.ModelResponse, error) {
			fmt.Printf("[tokens] input=%d output=%d\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)

			return ctx, resp, nil
		}),
		agent.WithOnToolCall(func(ctx context.Context, call kit.ToolCall) (context.Context, kit.ToolCall, error) {
			toolStartTimes[call.ID] = time.Now()

			return ctx, call, nil
		}),
		agent.WithOnToolResult(func(ctx context.Context, result kit.ToolResult) (context.Context, kit.ToolResult, error) {
			if start, ok := toolStartTimes[result.Call.ID]; ok {
				fmt.Printf("[tool]   %s took %dms\n", result.Call.Name, time.Since(start).Milliseconds())
				delete(toolStartTimes, result.Call.ID)
			}

			return ctx, result, nil
		}),
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

	fmt.Println()
	fmt.Println(result.Output.TextValue())
	fmt.Printf("\n[total]  input=%d output=%d\n", result.Usage.InputTokens, result.Usage.OutputTokens)
}
