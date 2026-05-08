package kit

import "testing"

func TestNewModelContentDeltaEvent(t *testing.T) {
	event := NewModelContentDeltaEvent(NewTextContent("hel"))

	if event.Type != EventContentDelta {
		t.Fatalf("Type = %q, want %q", event.Type, EventContentDelta)
	}

	if event.Content == nil || event.Content.Text == nil {
		t.Fatal("Content.Text is nil")
	}

	if event.Content.Text.Text != "hel" {
		t.Fatalf("delta text = %q, want hel", event.Content.Text.Text)
	}
}
