package middleware_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kit/kittest"
	"github.com/vitaliiPsl/crappy-adk/kit/middleware"
)

func retryableErr(msg string) *kit.LLMError {
	return &kit.LLMError{Kind: kit.ErrRateLimit, Message: msg, Retryable: true}
}

func nonRetryableErr(msg string) *kit.LLMError {
	return &kit.LLMError{Kind: kit.ErrAuthentication, Message: msg, Retryable: false}
}

func fastRetryOpts() []middleware.RetryOption {
	return []middleware.RetryOption{
		middleware.WithBaseDelay(time.Millisecond),
		middleware.WithMaxDelay(time.Millisecond),
	}
}

// --- Generate tests ---

func TestRetry_Generate_Success(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Text: "hello"},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	resp, err := wrapped.Generate(context.Background(), kit.ModelRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if got := resp.Message.Text(); got != "hello" {
		t.Errorf("text = %q, want %q", got, "hello")
	}

	model.AssertCallCount(t, 1)
}

func TestRetry_Generate_RetryThenSuccess(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Error: retryableErr("attempt 1")},
		kittest.Turn{Error: retryableErr("attempt 2")},
		kittest.Turn{Text: "recovered"},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	resp, err := wrapped.Generate(context.Background(), kit.ModelRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if got := resp.Message.Text(); got != "recovered" {
		t.Errorf("text = %q, want %q", got, "recovered")
	}

	model.AssertCallCount(t, 3)
}

func TestRetry_Generate_RetriesExhausted(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Error: retryableErr("fail 1")},
		kittest.Turn{Error: retryableErr("fail 2")},
		kittest.Turn{Error: retryableErr("fail 3")},
	)

	wrapped := middleware.Retry(model,
		append(fastRetryOpts(), middleware.WithMaxAttempts(3))...,
	)

	_, err := wrapped.Generate(context.Background(), kit.ModelRequest{})
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, kit.ErrRateLimit) {
		t.Errorf("error = %v, want ErrRateLimit", err)
	}

	model.AssertCallCount(t, 3)
}

func TestRetry_Generate_NonRetryable(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Error: nonRetryableErr("bad auth")},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	_, err := wrapped.Generate(context.Background(), kit.ModelRequest{})
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, kit.ErrAuthentication) {
		t.Errorf("error = %v, want ErrAuthentication", err)
	}

	model.AssertCallCount(t, 1)
}

func TestRetry_Generate_ContextCancelled(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Error: retryableErr("fail")},
		kittest.Turn{Text: "should not reach"},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	wrapped := middleware.Retry(model,
		middleware.WithBaseDelay(time.Second),
		middleware.WithMaxDelay(time.Second),
	)

	_, err := wrapped.Generate(ctx, kit.ModelRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestRetry_GenerateStream_Success(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Text: "hello world"},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	stream, err := wrapped.GenerateStream(context.Background(), kit.ModelRequest{})
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}

	got := collectText(t, stream)
	if got != "hello world" {
		t.Errorf("text = %q, want %q", got, "hello world")
	}

	model.AssertCallCount(t, 1)
}

func TestRetry_GenerateStream_ImmediateRetryThenSuccess(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Error: retryableErr("transient")},
		kittest.Turn{Text: "recovered"},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	stream, err := wrapped.GenerateStream(context.Background(), kit.ModelRequest{})
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}

	got := collectText(t, stream)
	if got != "recovered" {
		t.Errorf("text = %q, want %q", got, "recovered")
	}

	model.AssertCallCount(t, 2)
}

func TestRetry_GenerateStream_ImmediateRetriesExhausted(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Error: retryableErr("fail 1")},
		kittest.Turn{Error: retryableErr("fail 2")},
	)

	wrapped := middleware.Retry(model,
		append(fastRetryOpts(), middleware.WithMaxAttempts(2))...,
	)

	_, err := wrapped.GenerateStream(context.Background(), kit.ModelRequest{})
	if err == nil {
		t.Fatal("expected error from GenerateStream")
	}

	if !errors.Is(err, kit.ErrRateLimit) {
		t.Errorf("error = %v, want ErrRateLimit", err)
	}
}

func TestRetry_GenerateStream_ImmediateNonRetryable(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Error: nonRetryableErr("bad auth")},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	_, err := wrapped.GenerateStream(context.Background(), kit.ModelRequest{})
	if err == nil {
		t.Fatal("expected error from GenerateStream")
	}

	if !errors.Is(err, kit.ErrAuthentication) {
		t.Errorf("error = %v, want ErrAuthentication", err)
	}

	model.AssertCallCount(t, 1)
}

func TestRetry_GenerateStream_PreChunkRetryThenSuccess(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Stream: []kittest.ChunkResult{
			{Err: retryableErr("transient")},
		}},
		kittest.Turn{Text: "recovered"},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	stream, err := wrapped.GenerateStream(context.Background(), kit.ModelRequest{})
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}

	got := collectText(t, stream)
	if got != "recovered" {
		t.Errorf("text = %q, want %q", got, "recovered")
	}

	model.AssertCallCount(t, 2)
}

func TestRetry_GenerateStream_PreChunkRetriesExhausted(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Stream: []kittest.ChunkResult{
			{Err: retryableErr("fail 1")},
		}},
		kittest.Turn{Stream: []kittest.ChunkResult{
			{Err: retryableErr("fail 2")},
		}},
	)

	wrapped := middleware.Retry(model,
		append(fastRetryOpts(), middleware.WithMaxAttempts(2))...,
	)

	stream, err := wrapped.GenerateStream(context.Background(), kit.ModelRequest{})
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}

	// Error surfaces through the stream iterator.
	for _, iterErr := range stream.Iter() {
		if iterErr != nil && errors.Is(iterErr, kit.ErrRateLimit) {
			return // expected
		}
	}

	t.Fatal("expected ErrRateLimit from stream iteration")
}

func TestRetry_GenerateStream_MidStreamPassesThrough(t *testing.T) {
	model := kittest.NewModel(t,
		kittest.Turn{Stream: []kittest.ChunkResult{
			{Chunk: kit.NewTextChunk("partial")},
			{Err: retryableErr("mid-stream failure")},
		}},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	stream, err := wrapped.GenerateStream(context.Background(), kit.ModelRequest{})
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}

	var (
		sawText  bool
		sawError bool
	)

	for chunk, iterErr := range stream.Iter() {
		if iterErr != nil {
			sawError = true

			if !errors.Is(iterErr, kit.ErrRateLimit) {
				t.Errorf("stream error = %v, want ErrRateLimit", iterErr)
			}

			break
		}

		if chunk.Type == kit.ChunkTypeText {
			sawText = true
		}
	}

	if !sawText {
		t.Error("expected text chunk before error")
	}

	if !sawError {
		t.Error("expected mid-stream error to pass through")
	}

	model.AssertCallCount(t, 1)
}

func TestRetry_GenerateStream_MixedImmediateAndPreChunkRetries(t *testing.T) {
	model := kittest.NewModel(t,
		// attempt 0: immediate error
		kittest.Turn{Error: retryableErr("immediate fail")},
		// attempt 1: stream acquired, pre-chunk error
		kittest.Turn{Stream: []kittest.ChunkResult{
			{Err: retryableErr("pre-chunk fail")},
		}},
		// attempt 2: success
		kittest.Turn{Text: "finally"},
	)

	wrapped := middleware.Retry(model, fastRetryOpts()...)

	stream, err := wrapped.GenerateStream(context.Background(), kit.ModelRequest{})
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}

	got := collectText(t, stream)
	if got != "finally" {
		t.Errorf("text = %q, want %q", got, "finally")
	}

	model.AssertCallCount(t, 3)
}

func collectText(t *testing.T, stream *kit.ModelStream) string {
	t.Helper()

	var text strings.Builder

	for chunk, err := range stream.Iter() {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}

		if chunk.Type == kit.ChunkTypeText {
			text.WriteString(chunk.Text)
		}
	}

	return text.String()
}
