package tool

import (
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kittest"
)

func TestSetAddKeepsFirstAndPreservesOrder(t *testing.T) {
	set := NewSet(kittest.NewTool(t, "search", "v1"), kittest.NewTool(t, "calc", ""))
	set.Add(kittest.NewTool(t, "search", "v2"))

	tools := set.List()
	if len(tools) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(tools))
	}

	if tools[0].Definition().Description != "v1" {
		t.Fatalf("tools[0] description = %q, want v1 (first wins)", tools[0].Definition().Description)
	}

	if tools[1].Definition().Name != "calc" {
		t.Fatalf("tools[1] name = %q, want calc", tools[1].Definition().Name)
	}
}

func TestSetGet(t *testing.T) {
	set := NewSet(kittest.NewTool(t, "search", "v1"))

	tool, ok := set.Get("search")
	if !ok {
		t.Fatal("Get(search) = false, want true")
	}

	if tool.Definition().Description != "v1" {
		t.Fatalf("description = %q, want v1", tool.Definition().Description)
	}

	if _, ok := set.Get("missing"); ok {
		t.Fatal("Get(missing) = true, want false")
	}
}
