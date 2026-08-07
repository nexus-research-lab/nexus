package orchestration

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsTransientMutationError(t *testing.T) {
	for _, err := range []error{
		ErrVersionConflict,
		fmt.Errorf("wrapped: %w", ErrVersionConflict),
		errors.New("database is locked (SQLITE_BUSY)"),
		errors.New("database table is locked"),
		errors.New("SQLITE_LOCKED: shared cache conflict"),
	} {
		if !IsTransientMutationError(err) {
			t.Fatalf("error %q was not classified as transient", err)
		}
	}
	if IsTransientMutationError(ErrInvariant) ||
		IsTransientMutationError(errors.New("constraint failed")) {
		t.Fatal("deterministic invariant error was classified as transient")
	}
}
