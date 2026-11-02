package orchestration

import (
	"context"
	"testing"
)

func TestRecoveryExecutionsScanUsesActivePlanAndCursor(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b", "unmanaged"} {
		if _, err := repository.Create(ctx, createTestCommand(id)); err != nil {
			t.Fatal(err)
		}
		if id != "unmanaged" {
			if _, err := repository.WritePlan(ctx, testPlanCommand(id, 1, id, "", 1)); err != nil {
				t.Fatal(err)
			}
		}
	}
	first, err := repository.ListRecoveryExecutions(ctx, "", 1)
	if err != nil || len(first) != 1 || first[0].ID != "execution-a" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	next, err := repository.ListRecoveryExecutions(ctx, first[0].ID, 64)
	if err != nil || len(next) != 1 || next[0].ID != "execution-b" {
		t.Fatalf("next=%+v err=%v", next, err)
	}
}
