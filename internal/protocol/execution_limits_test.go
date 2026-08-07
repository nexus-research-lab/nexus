package protocol

import (
	"errors"
	"testing"
	"time"
)

func TestValidateExecutionProjectionLimitBoundary(t *testing.T) {
	if err := ValidateExecutionProjectionLimit(
		"input_refs",
		ExecutionProjectionCollectionLimit,
	); err != nil {
		t.Fatalf("limit boundary rejected: %v", err)
	}
	err := ValidateExecutionProjectionLimit(
		"input_refs",
		ExecutionProjectionCollectionLimit+1,
	)
	if !errors.Is(err, ErrExecutionProjectionLimitExceeded) {
		t.Fatalf("error = %v", err)
	}
	var typed *ExecutionProjectionLimitError
	if !errors.As(err, &typed) ||
		typed.Field != "input_refs" ||
		typed.Count != ExecutionProjectionCollectionLimit+1 ||
		typed.Limit != ExecutionProjectionCollectionLimit {
		t.Fatalf("typed error = %#v", typed)
	}
}

func TestValidSubagentReconciliationDeadlineRequiresExactGrace(t *testing.T) {
	exitedAt := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	if !ValidSubagentReconciliationDeadline(
		exitedAt,
		exitedAt.Add(SubagentReconciliationGrace),
	) {
		t.Fatal("exact reconciliation grace was rejected")
	}
	if ValidSubagentReconciliationDeadline(
		exitedAt,
		exitedAt.Add(SubagentReconciliationGrace+time.Nanosecond),
	) {
		t.Fatal("non-exact reconciliation grace was accepted")
	}
}
