// INPUT: Repository Plan commands containing typed output claims.
// OUTPUT: Canonical persistence-bound claims or ErrInvariant rejection.
// POS: Storage defense-in-depth coverage for protocol-owned output scope semantics.
package orchestration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNormalizeAndValidatePlanCanonicalizesOutputClaims(t *testing.T) {
	command := testPlanCommand("scope-canonical", 1, "scope-canonical", "", 1)
	command.WorkItems[0].OutputClaims[0].Scope = " dir:output//research/ "
	command.WorkItems[0].OutputClaims[0].Mode = ""

	items, _, err := normalizeAndValidatePlan(
		command.Plan,
		command.WorkItems,
		command.Dependencies,
		time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("normalize Plan: %v", err)
	}
	got := items[0].OutputClaims[0]
	if got.Scope != "dir:output/research" || got.Mode != protocol.WorkOutputScopeExclusive {
		t.Fatalf("canonical claim = %#v", got)
	}
	if command.WorkItems[0].OutputClaims[0].Scope != " dir:output//research/ " {
		t.Fatalf("source command was mutated: %#v", command.WorkItems[0].OutputClaims[0])
	}
}

func TestWritePlanPersistsCanonicalOutputClaims(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	if _, err := repository.Create(ctx, createTestCommand("scope-persisted")); err != nil {
		t.Fatal(err)
	}
	command := testPlanCommand("scope-persisted", 1, "scope-persisted", "", 1)
	command.WorkItems[0].OutputClaims[0].Scope = " dir:output//research/ "
	command.WorkItems[0].OutputClaims[0].Mode = ""

	snapshot, err := repository.WritePlan(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	wantWorkItemID := command.WorkItems[0].WorkItem.ID
	for _, claim := range snapshot.OutputClaims {
		if claim.WorkItemID != wantWorkItemID {
			continue
		}
		if claim.Scope != "dir:output/research" ||
			claim.Mode != protocol.WorkOutputScopeExclusive {
			t.Fatalf("persisted claim = %#v", claim)
		}
		return
	}
	t.Fatalf("output claim for %s not found in snapshot: %#v", wantWorkItemID, snapshot.OutputClaims)
}

func TestNormalizeAndValidatePlanOutputScopeConflictMatrix(t *testing.T) {
	exclusive := protocol.WorkOutputScopeExclusive
	shared := protocol.WorkOutputScopeShared
	tests := []struct {
		name      string
		left      string
		leftMode  protocol.WorkOutputScopeMode
		right     string
		rightMode protocol.WorkOutputScopeMode
		wantErr   bool
	}{
		{name: "same file exclusive exclusive", left: "file:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: exclusive, wantErr: true},
		{name: "same file exclusive shared", left: "file:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: shared, wantErr: true},
		{name: "same file shared exclusive", left: "file:web/main.go", leftMode: shared, right: "file:web/main.go", rightMode: exclusive, wantErr: true},
		{name: "same file shared", left: "file:web/main.go", leftMode: shared, right: "file:web/main.go", rightMode: shared},
		{name: "different files", left: "file:web/main.go", leftMode: exclusive, right: "file:web/app.go", rightMode: exclusive},
		{name: "directory contains file", left: "dir:web", leftMode: exclusive, right: "file:web/src/main.go", rightMode: shared, wantErr: true},
		{name: "file inside directory reverse", left: "file:web/src/main.go", leftMode: shared, right: "dir:web", rightMode: exclusive, wantErr: true},
		{name: "directory equals file", left: "dir:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: exclusive, wantErr: true},
		{name: "directory contains directory", left: "dir:web", leftMode: shared, right: "dir:web/src", rightMode: exclusive, wantErr: true},
		{name: "child directory reverse", left: "dir:web/src", leftMode: exclusive, right: "dir:web", rightMode: shared, wantErr: true},
		{name: "directory shared", left: "dir:web", leftMode: shared, right: "dir:web/src", rightMode: shared},
		{name: "directory boundary", left: "dir:web/src", leftMode: exclusive, right: "file:web/src2/main.go", rightMode: exclusive},
		{name: "file is not ancestor", left: "file:web", leftMode: exclusive, right: "file:web/src/main.go", rightMode: exclusive},
		{name: "case-folded file", left: "file:Web/Final.md", leftMode: exclusive, right: "file:web/final.MD", rightMode: shared, wantErr: true},
		{name: "NFC directory ancestor", left: "dir:Réports", leftMode: exclusive, right: "file:re\u0301ports/final.md", rightMode: shared, wantErr: true},
		{name: "same semantic exclusive exclusive", left: "semantic:report", leftMode: exclusive, right: "semantic:report", rightMode: exclusive, wantErr: true},
		{name: "same semantic exclusive shared", left: "semantic:report", leftMode: exclusive, right: "semantic:report", rightMode: shared, wantErr: true},
		{name: "same semantic shared exclusive", left: "semantic:report", leftMode: shared, right: "semantic:report", rightMode: exclusive, wantErr: true},
		{name: "same semantic shared", left: "semantic:report", leftMode: shared, right: "semantic:report", rightMode: shared},
		{name: "different semantic", left: "semantic:report", leftMode: exclusive, right: "semantic:appendix", rightMode: exclusive},
		{name: "semantic case sensitive", left: "semantic:Report", leftMode: exclusive, right: "semantic:report", rightMode: exclusive},
		{name: "semantic and path", left: "semantic:web", leftMode: exclusive, right: "dir:web", rightMode: exclusive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := testPlanCommand("scope-matrix", 1, "scope-matrix", "", 1)
			command.WorkItems[0].OutputClaims[0].Scope = test.left
			command.WorkItems[0].OutputClaims[0].Mode = test.leftMode
			command.WorkItems[1].OutputClaims[0].Scope = test.right
			command.WorkItems[1].OutputClaims[0].Mode = test.rightMode
			_, _, err := normalizeAndValidatePlan(
				command.Plan,
				command.WorkItems,
				command.Dependencies,
				time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
			)
			if test.wantErr {
				if !errors.Is(err, ErrInvariant) {
					t.Fatalf("error = %v, want ErrInvariant", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid scopes rejected: %v", err)
			}
		})
	}
}

func TestNormalizeAndValidatePlanAllowsHardOrderedOutputScopeHandoff(t *testing.T) {
	command := testPlanCommand("scope-handoff", 1, "scope-handoff", "", 1)
	for index := range command.WorkItems {
		command.WorkItems[index].OutputClaims[0].Scope = "file:output/workgraph-demo.md"
	}
	command.Dependencies = []protocol.ExecutionPlanDependency{{
		WorkItemID:          command.WorkItems[1].WorkItem.ID,
		DependsOnWorkItemID: command.WorkItems[0].WorkItem.ID,
		Kind:                protocol.WorkDependencyHard,
	}}
	if _, _, err := normalizeAndValidatePlan(
		command.Plan,
		command.WorkItems,
		command.Dependencies,
		time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("hard-ordered output handoff rejected: %v", err)
	}

	command.Dependencies[0].Kind = protocol.WorkDependencySoft
	if _, _, err := normalizeAndValidatePlan(
		command.Plan,
		command.WorkItems,
		command.Dependencies,
		time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	); !errors.Is(err, ErrInvariant) {
		t.Fatalf("soft-only output overlap error = %v, want ErrInvariant", err)
	}
}

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

func TestOrderedOutputHandoffMigrationDropsLegacyUniqueIndexInBothDialects(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		path := filepath.Join(
			orchestrationMigrationDir(t, dialect),
			"00102_execution_ordered_output_handoffs.sql",
		)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		normalized := strings.ToLower(strings.Join(strings.Fields(string(body)), " "))
		if !strings.Contains(
			normalized,
			"drop index if exists uq_execution_plan_exclusive_output_claim",
		) {
			t.Fatalf("%s migration does not drop the legacy unique index", dialect)
		}
	}
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

func TestNormalizeAndValidatePlanAllowsUndeclaredOutputScope(t *testing.T) {
	command := testPlanCommand("scope-optional", 1, "scope-optional", "", 1)
	command.WorkItems[0].OutputClaims = nil

	if _, _, err := normalizeAndValidatePlan(
		command.Plan,
		command.WorkItems,
		command.Dependencies,
		time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("normalize plan without declared output scope: %v", err)
	}
}
