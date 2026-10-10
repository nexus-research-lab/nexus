// INPUT: Repository Plan commands containing typed output claims.
// OUTPUT: Canonical persistence-bound claims or ErrInvariant rejection.
// POS: Storage defense-in-depth coverage for protocol-owned output scope semantics.
package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestWritePlanAllowsHardOrderedOutputScopeHandoffAndRejectsConcurrentConflict(t *testing.T) {
	t.Run("all-hard ordered handoff", func(t *testing.T) {
		repository := newRepositoryTestStore(t)
		ctx := context.Background()
		if _, err := repository.Create(ctx, createTestCommand("scope-handoff-db")); err != nil {
			t.Fatal(err)
		}
		command := testPlanCommand("scope-handoff-db", 1, "scope-handoff-db", "", 1)
		command.WorkItems = append(command.WorkItems, testPlanWork(
			command.ExecutionID,
			command.Plan.ID,
			"work-scope-handoff-db-3",
			"spec-scope-handoff-db-3",
			2,
		))
		for index := range command.WorkItems {
			command.WorkItems[index].OutputClaims[0].Scope = "file:output/workgraph-demo.md"
		}
		command.Dependencies = []protocol.ExecutionPlanDependency{
			{
				WorkItemID:          command.WorkItems[1].WorkItem.ID,
				DependsOnWorkItemID: command.WorkItems[0].WorkItem.ID,
				Kind:                protocol.WorkDependencyHard,
			},
			{
				WorkItemID:          command.WorkItems[2].WorkItem.ID,
				DependsOnWorkItemID: command.WorkItems[1].WorkItem.ID,
				Kind:                protocol.WorkDependencyHard,
			},
		}

		snapshot, err := repository.WritePlan(ctx, command)
		if err != nil {
			t.Fatalf("persist hard-ordered output handoff: %v", err)
		}
		claimCount := 0
		for _, claim := range snapshot.OutputClaims {
			if claim.Scope == "file:output/workgraph-demo.md" &&
				claim.Mode == protocol.WorkOutputScopeExclusive {
				claimCount++
			}
		}
		if claimCount != 3 {
			t.Fatalf("persisted ordered exclusive claims = %d, want 3", claimCount)
		}
	})

	t.Run("unordered concurrent ownership", func(t *testing.T) {
		repository := newRepositoryTestStore(t)
		ctx := context.Background()
		if _, err := repository.Create(ctx, createTestCommand("scope-conflict-db")); err != nil {
			t.Fatal(err)
		}
		command := testPlanCommand("scope-conflict-db", 1, "scope-conflict-db", "", 1)
		for index := range command.WorkItems {
			command.WorkItems[index].OutputClaims[0].Scope = "file:output/workgraph-demo.md"
		}

		if _, err := repository.WritePlan(ctx, command); !errors.Is(err, ErrInvariant) {
			t.Fatalf("unordered output overlap error = %v, want ErrInvariant", err)
		}
		current, err := repository.Get(ctx, command.ExecutionID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Version != 1 {
			t.Fatalf("rejected Plan changed Execution version to %d", current.Version)
		}
	})
}

func TestNormalizeAndValidatePlanRejectsCaseFoldedDuplicateOnSameWorkItem(t *testing.T) {
	command := testPlanCommand("scope-case-duplicate", 1, "scope-case-duplicate", "", 1)
	claim := command.WorkItems[0].OutputClaims[0]
	claim.Scope = "file:Report/Résumé.md"
	command.WorkItems[0].OutputClaims = []protocol.ExecutionPlanOutputClaim{
		claim,
		{
			Scope: "file:report/re\u0301sume\u0301.MD",
			Mode:  protocol.WorkOutputScopeShared,
		},
	}
	_, _, err := normalizeAndValidatePlan(
		command.Plan,
		command.WorkItems,
		command.Dependencies,
		time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	)
	if !errors.Is(err, ErrInvariant) {
		t.Fatalf("error = %v, want ErrInvariant", err)
	}
}

func TestNormalizeAndValidatePlanRejectsInvalidOutputClaims(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*WritePlanCommand)
	}{
		{
			name: "untyped",
			mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims[0].Scope = "output/research"
			},
		},
		{
			name: "unknown kind",
			mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims[0].Scope = "resource:output/research"
			},
		},
		{
			name: "empty typed path",
			mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims[0].Scope = "file:"
			},
		},
		{
			name: "absolute path",
			mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims[0].Scope = "dir:/workspace"
			},
		},
		{
			name: "dot path",
			mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims[0].Scope = "dir:."
			},
		},
		{
			name: "parent escape",
			mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims[0].Scope = "file:../outside"
			},
		},
		{
			name: "backslash",
			mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims[0].Scope = `file:web\main.go`
			},
		},
		{
			name: "invalid mode",
			mutate: func(command *WritePlanCommand) {
				command.WorkItems[0].OutputClaims[0].Mode = "private"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := testPlanCommand("scope-invalid", 1, "scope-invalid", "", 1)
			test.mutate(&command)
			_, _, err := normalizeAndValidatePlan(
				command.Plan,
				command.WorkItems,
				command.Dependencies,
				time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
			)
			if !errors.Is(err, ErrInvariant) {
				t.Fatalf("error = %v, want ErrInvariant", err)
			}
		})
	}
}
