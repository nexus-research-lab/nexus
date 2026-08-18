package runtimecommand

import (
	"slices"
	"testing"
)

func TestRoundResourcesOwnsCleanupUntilPhysicalRoundClose(t *testing.T) {
	resources := NewRoundResources()
	var cleaned []string
	resources.Add(func() { cleaned = append(cleaned, "input") })
	resources.Add(func() { cleaned = append(cleaned, "capability") })
	if len(cleaned) != 0 {
		t.Fatalf("resources cleaned before round close: %v", cleaned)
	}
	resources.Close()
	resources.Close()
	if !slices.Equal(cleaned, []string{"capability", "input"}) {
		t.Fatalf("cleanup order = %v", cleaned)
	}
	resources.Add(func() { cleaned = append(cleaned, "late") })
	if !slices.Equal(cleaned, []string{"capability", "input", "late"}) {
		t.Fatalf("late cleanup was not immediate: %v", cleaned)
	}
}
