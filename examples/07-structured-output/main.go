/*
Example 07 — Structured output

Shows how an agent can call tools before returning a typed, validated result.

Run:

	go run ./examples/07-structured-output

Prerequisites:

	OPENAI_API_KEY must be set.
*/
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/providers"
	"github.com/vitaliiPsl/crappy-adk/providers/openai"
	"github.com/vitaliiPsl/crappy-adk/x/memory"
	"github.com/vitaliiPsl/crappy-adk/x/output"
	"github.com/vitaliiPsl/crappy-adk/x/tool"
)

type checkServiceInput struct {
	Service string `json:"service" jsonschema:"Service to inspect"`
}

type serviceMetrics struct {
	ErrorRatePercent float64 `json:"error_rate_percent"`
	P99LatencyMS     int     `json:"p99_latency_ms"`
}

type assessment struct {
	Status         string `json:"status" jsonschema:"One of healthy, degraded, or unavailable"`
	Summary        string `json:"summary" jsonschema:"Concise explanation supported by the metrics"`
	NeedsAttention bool   `json:"needs_attention" jsonschema:"Whether a person needs to investigate"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is not set")
	}

	checkService := tool.MustNew(
		"check_service",
		"Read current health metrics for a service.",
		func(_ *kit.RunContext, _ checkServiceInput) (serviceMetrics, error) {
			return serviceMetrics{
				ErrorRatePercent: 7.4,
				P99LatencyMS:     1850,
			}, nil
		},
	)

	model, err := openai.New("gpt-5.4-nano", providers.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("failed to create model: %v", err)
	}

	schema := output.MustFor[assessment](
		"service_assessment",
		"The completed service health assessment",
	)

	a, err := agent.New(
		model,
		memory.NewHistory(),
		tool.NewSet(checkService),
		agent.WithInstructions(
			"Use check_service before assessing health. Base every conclusion on the returned metrics.",
		),
		agent.WithOutputSchema(schema),
	)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	response, err := a.Run(
		context.Background(),
		kit.NewUserMessage(kit.NewTextContent("Assess the checkout service.")),
	)
	if errors.Is(err, kit.ErrInvalidOutput) {
		log.Fatalf("model returned an invalid assessment: %v", err)
	}

	if err != nil {
		log.Fatalf("agent run failed: %v", err)
	}

	result, err := output.Decode[assessment](response)
	if err != nil {
		log.Fatalf("failed to decode assessment: %v", err)
	}

	fmt.Printf("status: %s\n", result.Status)
	fmt.Printf("summary: %s\n", result.Summary)
	fmt.Printf("needs attention: %t\n", result.NeedsAttention)
}
