package memory

import (
	"context"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

func TestHistoryContextReturnsInitialMessages(t *testing.T) {
	msg := kit.NewUserMessage(kit.NewTextContent("hello"))
	mem := NewHistory(msg)

	got, err := mem.Context(context.Background())
	if err != nil {
		t.Fatalf("Context: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(Context) = %d, want 1", len(got))
	}

	if got[0].TextContent().Text != "hello" {
		t.Fatalf("Context()[0] = %q, want hello", got[0].TextContent().Text)
	}
}

func TestHistoryRecordAppendsMessages(t *testing.T) {
	mem := NewHistory()

	first := kit.NewUserMessage(kit.NewTextContent("first"))
	second := kit.NewModelMessage(kit.NewTextContent("second"))

	if err := mem.Record(context.Background(), first); err != nil {
		t.Fatalf("Record first: %v", err)
	}

	if err := mem.Record(context.Background(), second); err != nil {
		t.Fatalf("Record second: %v", err)
	}

	got, err := mem.History(context.Background())
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(History) = %d, want 2", len(got))
	}

	if got[0].TextContent().Text != "first" || got[1].TextContent().Text != "second" {
		t.Fatalf("History = %+v, want first then second", got)
	}
}

func TestHistoryContextStartsAtLatestSummary(t *testing.T) {
	old := kit.NewUserMessage(kit.NewTextContent("old"))
	firstSummary := kit.NewUserMessage(kit.NewSummaryContent("first summary"))

	middle := kit.NewUserMessage(kit.NewTextContent("middle"))

	latestSummary := kit.NewUserMessage(kit.NewSummaryContent("latest summary"))
	recent := kit.NewUserMessage(kit.NewTextContent("recent"))

	mem := NewHistory(old, firstSummary, middle, latestSummary, recent)

	got, err := mem.Context(context.Background())
	if err != nil {
		t.Fatalf("Context: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(Context) = %d, want latest summary + recent", len(got))
	}

	if got[0].Content[0].Summary.Text != "latest summary" {
		t.Fatalf("Context[0] summary = %q, want latest summary", got[0].Content[0].Summary.Text)
	}

	if got[1].TextContent().Text != "recent" {
		t.Fatalf("Context[1] = %q, want recent", got[1].TextContent().Text)
	}

	history, err := mem.History(context.Background())
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	if len(history) != 5 {
		t.Fatalf("len(History) = %d, want full history length 5", len(history))
	}
}
