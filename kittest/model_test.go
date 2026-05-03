package kittest

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestModel_Generate(t *testing.T) {
	wantErr := errors.New("boom")
	model := NewModel(t, ModelResult{
		Response: kit.ModelResponse{
			Message:      kit.NewModelMessage([]kit.Content{kit.NewTextContent("hello")}),
			FinishReason: kit.FinishReasonStop,
		},
		Error: wantErr,
	})
	request := kit.ModelRequest{
		Instructions: "be helpful",
		Messages: []kit.Message{
			kit.NewUserMessage([]kit.Content{kit.NewTextContent("hi")}),
		},
	}

	response, err := model.Generate(context.Background(), request)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	if got := response.Message.TextContent().Text; got != "hello" {
		t.Fatalf("response text = %q, want hello", got)
	}

	model.AssertCalledWith(t, 0, request)
}

func TestModel_Generate_ConsumesResultsInOrder(t *testing.T) {
	model := NewModel(t,
		ModelResult{Response: kit.ModelResponse{FinishReason: kit.FinishReasonToolCall}},
		ModelResult{Response: kit.ModelResponse{FinishReason: kit.FinishReasonStop}},
	)

	first, err := model.Generate(context.Background(), kit.ModelRequest{Instructions: "first"})
	if err != nil || first.FinishReason != kit.FinishReasonToolCall {
		t.Fatalf("first Generate = %q, %v; want tool_call, nil", first.FinishReason, err)
	}

	second, err := model.Generate(context.Background(), kit.ModelRequest{Instructions: "second"})
	if err != nil || second.FinishReason != kit.FinishReasonStop {
		t.Fatalf("second Generate = %q, %v; want stop, nil", second.FinishReason, err)
	}

	model.AssertCallCount(t, 2)

	if got := model.CallAt(1).Instructions; got != "second" {
		t.Fatalf("CallAt(1).Instructions = %q, want second", got)
	}
}

func TestModel_AssertNeverCalled(t *testing.T) {
	model := NewModel(t)

	model.AssertNeverCalled(t)
}
