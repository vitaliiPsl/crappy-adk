package bash

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunBash_Success(t *testing.T) {
	out, err := runBash(context.Background(), "echo hello")
	if err != nil {
		t.Fatal(err)
	}

	if out != "hello" {
		t.Errorf("expected 'hello', got: %q", out)
	}
}

func TestRunBash_StderrIncludedOnError(t *testing.T) {
	_, err := runBash(context.Background(), "echo 'oops' >&2; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "oops") {
		t.Errorf("expected stderr in error, got: %v", err)
	}
}

func TestRunBash_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := runBash(ctx, "sleep 10")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRunBash_MultilineOutput(t *testing.T) {
	out, err := runBash(context.Background(), "printf 'a\nb\nc'")
	if err != nil {
		t.Fatal(err)
	}

	if out != "a\nb\nc" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestRunBash_UsesShellEnvVar(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")

	out, err := runBash(context.Background(), "echo fromsh")
	if err != nil {
		t.Fatal(err)
	}

	if out != "fromsh" {
		t.Errorf("expected 'fromsh', got: %q", out)
	}
}

func TestRunBash_FallsBackToShWhenNoShellEnv(t *testing.T) {
	t.Setenv("SHELL", "")

	out, err := runBash(context.Background(), "echo fallback")
	if err != nil {
		t.Fatal(err)
	}

	if out != "fallback" {
		t.Errorf("expected 'fallback', got: %q", out)
	}
}
