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
			Message:      kit.NewModelMessage(kit.NewTextContent("hello")),
			FinishReason: kit.FinishReasonStop,
		},
		Error: wantErr,
	})
	request := kit.ModelRequest{
		Instructions: "be helpful",
		Messages: []kit.Message{
			kit.NewUserMessage(kit.NewTextContent("hi")),
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

func TestModel_Stream(t *testing.T) {
	wantResp := kit.ModelResponse{
		Message:      kit.NewModelMessage(kit.NewTextContent("hello")),
		FinishReason: kit.FinishReasonStop,
	}
	events := []kit.ModelEvent{
		kit.NewModelContentStartedEvent(kit.NewTextContent("hello")),
		kit.NewModelContentDoneEvent(kit.NewTextContent("hello")),
	}
	model := NewModel(t, ModelResult{Events: events, Response: wantResp})
	request := kit.ModelRequest{Instructions: "be helpful"}

	stream := model.Stream(context.Background(), request)

	var got []kit.ModelEvent
	for event := range stream.Iter() {
		got = append(got, event)
	}

	if len(got) != len(events) {
		t.Fatalf("event count = %d, want %d", len(got), len(events))
	}

	if got[0].Type != kit.EventContentStarted || got[1].Type != kit.EventContentDone {
		t.Errorf("event types = %q/%q, want started/done", got[0].Type, got[1].Type)
	}

	resp, err := stream.Result()
	if err != nil {
		t.Fatalf("stream error = %v", err)
	}

	if resp.FinishReason != kit.FinishReasonStop {
		t.Errorf("finish reason = %q, want stop", resp.FinishReason)
	}

	model.AssertCalledWith(t, 0, request)
}

func TestModel_Stream_ReturnsError(t *testing.T) {
	wantErr := errors.New("model down")
	model := NewModel(t, ModelResult{Error: wantErr})

	stream := model.Stream(context.Background(), kit.ModelRequest{})

	_, err := stream.Result()
	if !errors.Is(err, wantErr) {
		t.Fatalf("stream error = %v, want %v", err, wantErr)
	}
}

func TestModel_Stream_CountedAsCall(t *testing.T) {
	model := NewModel(t,
		ModelResult{Response: kit.ModelResponse{FinishReason: kit.FinishReasonStop}},
		ModelResult{Response: kit.ModelResponse{FinishReason: kit.FinishReasonStop}},
	)

	if _, err := model.Generate(context.Background(), kit.ModelRequest{Instructions: "first"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	model.Stream(context.Background(), kit.ModelRequest{Instructions: "second"})

	model.AssertCallCount(t, 2)

	if got := model.CallAt(1).Instructions; got != "second" {
		t.Fatalf("CallAt(1).Instructions = %q, want second", got)
	}
}
