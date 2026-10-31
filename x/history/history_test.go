package history

import (
	"context"
	"errors"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestHistoryReturnsAppendedMessagesInOrder(t *testing.T) {
	ctx := context.Background()
	h := New()

	if err := h.Append(ctx, kit.NewUserMessage(kit.NewTextContent("first"))); err != nil {
		t.Fatal(err)
	}

	if err := h.Append(ctx, kit.NewModelMessage(kit.NewTextContent("second"))); err != nil {
		t.Fatal(err)
	}

	messages, err := h.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 2 || messages[0].TextContent().Text != "first" || messages[1].TextContent().Text != "second" {
		t.Fatalf("messages = %+v", messages)
	}
}

func TestHistoryMessagesReturnsCopy(t *testing.T) {
	ctx := context.Background()
	h := New()

	if err := h.Append(ctx, kit.NewUserMessage(kit.NewTextContent("stable"))); err != nil {
		t.Fatal(err)
	}

	messages, err := h.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}

	messages[0] = kit.Message{}

	reread, err := h.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if reread[0].TextContent().Text != "stable" {
		t.Fatalf("Messages exposed stored state: %+v", reread)
	}
}

func TestHistoryMessagesReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := New().Messages(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Messages() error = %v, want context canceled", err)
	}
}
