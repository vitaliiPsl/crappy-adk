package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
	xmemory "github.com/vitaliiPsl/crappy-adk/x/memory"
	"github.com/vitaliiPsl/crappy-adk/x/output"
	xtool "github.com/vitaliiPsl/crappy-adk/x/tool"
)

type assessment struct {
	Summary string `json:"summary"`
}

func TestRunReturnsStructuredOutput(t *testing.T) {
	schema := output.MustFor[assessment]("assessment", "Completed assessment")
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent(`{"summary":"healthy"}`)),
			FinishReason: kit.FinishReasonStop,
		},
	})

	a, err := New(model, xmemory.NewHistory(), xtool.NewSet(), WithOutputSchema(schema))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("check")))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := output.Decode[assessment](response)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if got.Summary != "healthy" {
		t.Fatalf("Summary = %q, want healthy", got.Summary)
	}

	if !reflect.DeepEqual(model.CallAt(0).OutputSchema, &schema) {
		t.Fatalf("request output schema = %+v, want configured schema", model.CallAt(0).OutputSchema)
	}
}

func TestRunValidatesOnlyFinalOutput(t *testing.T) {
	schema := output.MustFor[assessment]("assessment", "")
	call := kit.NewToolCall("call-1", "inspect", map[string]any{})
	tool := kittest.NewTool(t, "inspect", "inspect", kittest.ToolResult{Result: "healthy"})
	model := kittest.NewModel(t,
		kittest.ModelResult{Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewToolCallContent(call)),
			FinishReason: kit.FinishReasonToolCall,
		}},
		kittest.ModelResult{Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent(`{"summary":"healthy"}`)),
			FinishReason: kit.FinishReasonStop,
		}},
	)

	a, err := New(model, xmemory.NewHistory(), xtool.NewSet(tool), WithOutputSchema(schema))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("check"))); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if model.CallAt(0).OutputSchema == nil || model.CallAt(1).OutputSchema == nil {
		t.Fatal("output schema was not sent on every model call")
	}
}

func TestRunRejectsInvalidStructuredOutput(t *testing.T) {
	schema := output.MustFor[assessment]("assessment", "")
	model := kittest.NewModel(t, kittest.ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage(kit.NewTextContent(`{"unexpected":true}`)),
			FinishReason: kit.FinishReasonStop,
		},
	})

	a, err := New(model, xmemory.NewHistory(), xtool.NewSet(), WithOutputSchema(schema))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := a.Run(context.Background(), kit.NewUserMessage(kit.NewTextContent("check")))
	if !errors.Is(err, kit.ErrInvalidOutput) {
		t.Fatalf("Run error = %v, want invalid output", err)
	}

	if len(response.StructuredOutput) != 0 {
		t.Fatalf("StructuredOutput = %s, want empty", response.StructuredOutput)
	}
}
