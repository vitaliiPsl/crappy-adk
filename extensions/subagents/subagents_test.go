package subagents_test

import (
	"context"
	"strings"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/extensions/subagents"
	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
)

func TestWithSubAgents_EmptyReturnsNoOptions(t *testing.T) {
	if got := len(subagents.WithSubAgents()); got != 0 {
		t.Fatalf("len(options) = %d, want 0", got)
	}
}

func TestWithSubAgents_NilAgentReturnsConstructionError(t *testing.T) {
	model := kittest.NewModel(t)

	_, err := agent.New(model, agent.WithExtension(subagents.WithSubAgents(nil)))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "subagent cannot be nil") {
		t.Fatalf("err = %v", err)
	}
}

func TestWithSubAgents_MissingNameReturnsConstructionError(t *testing.T) {
	model := kittest.NewModel(t)

	sa, err := agent.New(model, agent.WithDescription("no name"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = agent.New(model, agent.WithExtension(subagents.WithSubAgents(sa)))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "subagent must have a name") {
		t.Fatalf("err = %v", err)
	}
}

func TestWithSubAgents_DuplicateNameReturnsConstructionError(t *testing.T) {
	model := kittest.NewModel(t)

	a, err := agent.New(model, agent.WithName("writer"), agent.WithDescription("first"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	b, err := agent.New(model, agent.WithName("writer"), agent.WithDescription("second"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = agent.New(model, agent.WithExtension(subagents.WithSubAgents(a, b)))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), `duplicate subagent name "writer"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestWithSubAgents_RegistersToolAndInstruction(t *testing.T) {
	model := kittest.NewModel(t, kittest.ModelTurn{Text: "done"})

	researcher, err := agent.New(
		model,
		agent.WithName("researcher"),
		agent.WithDescription("Reads the codebase."),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	parent, err := agent.New(model, agent.WithExtension(subagents.WithSubAgents(researcher)))
	if err != nil {
		t.Fatalf("New parent: %v", err)
	}

	_, err = parent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("hello")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	req := model.CallAt(0)
	if !strings.Contains(req.Instruction, "# Subagents") {
		t.Fatalf("instruction = %q", req.Instruction)
	}

	if !strings.Contains(req.Instruction, "- researcher: Reads the codebase.") {
		t.Fatalf("instruction = %q", req.Instruction)
	}

	found := false
	for _, tool := range req.Tools {
		if tool.Name == "agent" {
			found = true

			break
		}
	}

	if !found {
		t.Fatal("expected agent tool to be registered")
	}
}

func TestWithSubAgents_ToolExecutesSelectedSubagent(t *testing.T) {
	subagentModel := kittest.NewModel(t, kittest.ModelTurn{Text: "research result"})
	parentModel := kittest.NewModel(t,
		kittest.ModelTurn{ToolCalls: []kit.ToolCall{{
			ID:   "call_1",
			Name: "agent",
			Arguments: map[string]any{
				"agent":  "researcher",
				"prompt": "inspect the codebase",
			},
		}}},
		kittest.ModelTurn{Text: "parent result"},
	)

	researcher, err := agent.New(
		subagentModel,
		agent.WithName("researcher"),
		agent.WithDescription("Reads the codebase."),
	)
	if err != nil {
		t.Fatalf("New subagent: %v", err)
	}

	parent, err := agent.New(parentModel, agent.WithExtension(subagents.WithSubAgents(researcher)))
	if err != nil {
		t.Fatalf("New parent: %v", err)
	}

	got, err := parent.Run(context.Background(), []kit.Message{
		kit.NewUserMessage(kit.NewTextPart("delegate this")),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got.Output.Text != "parent result" {
		t.Fatalf("result = %q, want %q", got.Output.Text, "parent result")
	}

	req := subagentModel.CallAt(0)
	if len(req.Messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(req.Messages))
	}

	if req.Messages[0].Text() != "inspect the codebase" {
		t.Fatalf("prompt = %q, want %q", req.Messages[0].Text(), "inspect the codebase")
	}

	parentReq := parentModel.CallAt(1)
	if len(parentReq.Messages) != 3 {
		t.Fatalf("len(parent messages) = %d, want 3", len(parentReq.Messages))
	}

	if got := parentReq.Messages[2].Text(); got != "research result" {
		t.Fatalf("tool result = %q, want %q", got, "research result")
	}
}
