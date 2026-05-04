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

	model := openai.New("", modelID, openai.WithBaseURL(baseURL))

	a := agent.New(model, nil, kit.AgentConfig{
		Instructions: "You are a helpful assistant.",
	})

	resp, err := a.Run(context.Background(), []kit.Message{
		kit.NewUserMessage([]kit.Content{
			kit.NewTextContent("In one sentence, what is a transformer in machine learning?"),
		}),
	})
	if err != nil {
		log.Fatalf("agent run failed: %v", err)
	}

	fmt.Printf("[%s]\n%s\n", modelID, resp.Output.Text)
	fmt.Printf("\ntokens — in: %d, out: %d\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
}
