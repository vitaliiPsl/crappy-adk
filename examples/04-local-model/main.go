/*
Example 04 — Local model

Shows how to point the agent at a local inference server using the OpenAI
provider's WithBaseURL option. Both Ollama and LM Studio expose an
OpenAI-compatible API, so no provider-specific code is needed.

Run with Ollama (default):

	ollama pull gemma4
	go run ./examples/04-local-model

Run with another local server:

	LOCAL_BASE_URL=http://localhost:1234/v1 LOCAL_MODEL=gemma4 go run ./examples/04-local-model
*/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers/openai"
	"github.com/vitaliiPsl/crappy-adk/x/memory"
	tool "github.com/vitaliiPsl/crappy-adk/x/tool"
)

const (
	defaultBaseURL = "http://localhost:11434/v1"
	defaultModel   = "gemma4"
)

func main() {
	baseURL := os.Getenv("LOCAL_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	modelID := os.Getenv("LOCAL_MODEL")
	if modelID == "" {
		modelID = defaultModel
	}

	model, err := openai.New("", modelID, openai.WithBaseURL(baseURL))
	if err != nil {
		log.Fatalf("failed to create model: %v", err)
	}

	a, err := agent.New(model, memory.NewHistory(), tool.NewSet(), agent.WithInstructions("You are a helpful assistant."))
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	resp, err := a.Run(context.Background(), kit.NewUserMessage(
		kit.NewTextContent("In one sentence, what is a transformer in machine learning?")))
	if err != nil {
		log.Fatalf("agent run failed: %v", err)
	}

	fmt.Printf("[%s]\n%s\n", modelID, resp.Output.Text)
	fmt.Printf("\ntokens — in: %d, out: %d\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
}
