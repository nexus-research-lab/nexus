// INPUT: Plan drafts whose Work Items declare typed output scope pairs.
// OUTPUT: Canonical drafts or stable invalid_input/output_scope_conflict rejection.
// POS: Service integration matrix for the protocol-owned output scope contract.
package orchestration

import (
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNormalizeAndValidatePlanDraftOutputScopeConflictMatrix(t *testing.T) {
	exclusive := protocol.WorkOutputScopeExclusive
	shared := protocol.WorkOutputScopeShared
	tests := []struct {
		name      string
		left      string
		leftMode  protocol.WorkOutputScopeMode
		right     string
		rightMode protocol.WorkOutputScopeMode
		wantCode  ErrorCode
	}{
		{name: "same file exclusive exclusive", left: "file:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: exclusive, wantCode: ErrorCodeOutputScopeConflict},
		{name: "same file exclusive shared", left: "file:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: shared, wantCode: ErrorCodeOutputScopeConflict},
		{name: "same file shared exclusive", left: "file:web/main.go", leftMode: shared, right: "file:web/main.go", rightMode: exclusive, wantCode: ErrorCodeOutputScopeConflict},
		{name: "same file shared", left: "file:web/main.go", leftMode: shared, right: "file:web/main.go", rightMode: shared},
		{name: "different files", left: "file:web/main.go", leftMode: exclusive, right: "file:web/app.go", rightMode: exclusive},
		{name: "directory contains file", left: "dir:web", leftMode: exclusive, right: "file:web/src/main.go", rightMode: shared, wantCode: ErrorCodeOutputScopeConflict},
		{name: "file inside directory reverse", left: "file:web/src/main.go", leftMode: shared, right: "dir:web", rightMode: exclusive, wantCode: ErrorCodeOutputScopeConflict},
		{name: "directory equals file", left: "dir:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: exclusive, wantCode: ErrorCodeOutputScopeConflict},
		{name: "directory contains directory", left: "dir:web", leftMode: shared, right: "dir:web/src", rightMode: exclusive, wantCode: ErrorCodeOutputScopeConflict},
		{name: "child directory reverse", left: "dir:web/src", leftMode: exclusive, right: "dir:web", rightMode: shared, wantCode: ErrorCodeOutputScopeConflict},
		{name: "directory shared", left: "dir:web", leftMode: shared, right: "dir:web/src", rightMode: shared},
		{name: "directory boundary", left: "dir:web/src", leftMode: exclusive, right: "file:web/src2/main.go", rightMode: exclusive},
		{name: "file is not ancestor", left: "file:web", leftMode: exclusive, right: "file:web/src/main.go", rightMode: exclusive},
		{name: "same semantic exclusive exclusive", left: "semantic:report", leftMode: exclusive, right: "semantic:report", rightMode: exclusive, wantCode: ErrorCodeOutputScopeConflict},
		{name: "same semantic exclusive shared", left: "semantic:report", leftMode: exclusive, right: "semantic:report", rightMode: shared, wantCode: ErrorCodeOutputScopeConflict},
		{name: "same semantic shared exclusive", left: "semantic:report", leftMode: shared, right: "semantic:report", rightMode: exclusive, wantCode: ErrorCodeOutputScopeConflict},
		{name: "same semantic shared", left: "semantic:report", leftMode: shared, right: "semantic:report", rightMode: shared},
		{name: "different semantic", left: "semantic:report", leftMode: exclusive, right: "semantic:appendix", rightMode: exclusive},
		{name: "semantic and path", left: "semantic:web", leftMode: exclusive, right: "dir:web", rightMode: exclusive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			draft := validPlanDraft()
			draft.Items[1].DependsOn = nil
			draft.Items[0].OutputScopes = []protocol.WorkOutputScope{{
				Scope: test.left,
				Mode:  test.leftMode,
			}}
			draft.Items[1].OutputScopes = []protocol.WorkOutputScope{{
				Scope: test.right,
				Mode:  test.rightMode,
			}}
			_, err := NormalizeAndValidatePlanDraft(draft)
			if test.wantCode == "" {
				if err != nil {
					t.Fatalf("valid scopes rejected: %v", err)
				}
				return
			}
			assertDomainErrorCode(t, err, test.wantCode)
		})
	}
}

func TestNormalizeAndValidatePlanDraftAllowsHardOrderedOutputScopeHandoff(t *testing.T) {
	draft := validPlanDraft()
	draft.Items[0].OutputScopes = []protocol.WorkOutputScope{{
		Scope: "file:output/workgraph-demo.md",
	}}
	draft.Items[2].OutputScopes = []protocol.WorkOutputScope{{
		Scope: "file:output/workgraph-demo.md",
	}}
	if _, err := NormalizeAndValidatePlanDraft(draft); err != nil {
		t.Fatalf("transitive hard dependency handoff rejected: %v", err)
	}

	draft.Items[1].DependsOn[0].Kind = protocol.WorkDependencySoft
	assertDomainErrorCode(
		t,
		ValidatePlanDraft(draft),
		ErrorCodeOutputScopeConflict,
	)
}

func TestNormalizeAndValidatePlanDraftRejectsInvalidOutputScopeMatrix(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		mode  protocol.WorkOutputScopeMode
	}{
		{name: "untyped", scope: "web/main.go"},
		{name: "unknown kind", scope: "resource:web/main.go"},
		{name: "empty typed path", scope: "file:"},
		{name: "absolute path", scope: "dir:/workspace"},
		{name: "dot path", scope: "dir:."},
		{name: "parent escape", scope: "file:../outside"},
		{name: "backslash", scope: `file:web\main.go`},
		{name: "invalid mode", scope: "semantic:report", mode: "private"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			draft := validPlanDraft()
			draft.Items[0].OutputScopes[0] = protocol.WorkOutputScope{
				Scope: test.scope,
				Mode:  test.mode,
			}
			_, err := NormalizeAndValidatePlanDraft(draft)
			var domainErr *DomainError
			if !errors.As(err, &domainErr) || domainErr.Code != ErrorCodeInvalidInput {
				t.Fatalf("error = %v, want invalid_input", err)
			}
		})
	}
}
