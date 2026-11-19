package toolset

import (
	"slices"
	"testing"

	"github.com/vitaliiPsl/crappy-adk/kit"
	"github.com/vitaliiPsl/crappy-adk/kittest"
)

func TestSetPreservesRegistrationAndRejectsDuplicates(t *testing.T) {
	first := kittest.NewTool(t, "first", "")
	second := kittest.NewTool(t, "second", "")

	set := New()
	if err := set.Register(first, second); err != nil {
		t.Fatal(err)
	}

	exposed := set.Tools()
	if !slices.Equal(exposed, []kit.Tool{first, second}) {
		t.Fatalf("exposed tools = %v", exposed)
	}

	exposed[0] = nil

	if set.Tools()[0] != first {
		t.Fatal("caller changed the registry through its exposed slice")
	}

	if resolved, found := set.Lookup("second"); !found || resolved != second {
		t.Fatalf("lookup = %v, %v", resolved, found)
	}

	if _, found := set.Lookup("missing"); found {
		t.Fatal("unknown tool resolved")
	}

	if err := set.Register(first); err == nil {
		t.Fatal("duplicate registration succeeded")
	}
}
