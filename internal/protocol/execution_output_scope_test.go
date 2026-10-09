// INPUT: Raw typed output scopes and scope pairs across kind/mode combinations.
// OUTPUT: Canonical grammar and exhaustive overlap/conflict expectations.
// POS: Protocol truth-source tests shared by Execution Orchestration service and storage.
package protocol

import (
	"errors"
	"testing"
)

func TestNormalizeWorkOutputScopeRejectsInvalidGrammar(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		mode  WorkOutputScopeMode
	}{
		{name: "empty", scope: ""},
		{name: "untyped", scope: "reports/final"},
		{name: "unknown kind", scope: "resource:reports/final"},
		{name: "empty file", scope: "file:"},
		{name: "empty directory", scope: "dir:  "},
		{name: "empty semantic", scope: "semantic:  "},
		{name: "absolute", scope: "file:/etc/passwd"},
		{name: "windows absolute", scope: "dir:C:/workspace"},
		{name: "dot", scope: "dir:."},
		{name: "parent", scope: "file:../outside"},
		{name: "cleaned parent escape", scope: "dir:web/../../outside"},
		{name: "backslash", scope: `file:web\main.go`},
		{name: "path control", scope: "file:web/\nmain.go"},
		{name: "semantic control", scope: "semantic:report\nfinal"},
		{name: "invalid mode", scope: "semantic:report", mode: "private"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeWorkOutputScope(WorkOutputScope{
				Scope: test.scope,
				Mode:  test.mode,
			})
			if !errors.Is(err, ErrInvalidWorkOutputScope) {
				t.Fatalf("error = %v, want ErrInvalidWorkOutputScope", err)
			}
		})
	}
}

func TestWorkOutputScopesConflictMatrix(t *testing.T) {
	exclusive := WorkOutputScopeExclusive
	shared := WorkOutputScopeShared
	tests := []struct {
		name      string
		left      string
		leftMode  WorkOutputScopeMode
		right     string
		rightMode WorkOutputScopeMode
		want      bool
	}{
		{name: "same file exclusive exclusive", left: "file:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: exclusive, want: true},
		{name: "same file exclusive shared", left: "file:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: shared, want: true},
		{name: "same file shared exclusive", left: "file:web/main.go", leftMode: shared, right: "file:web/main.go", rightMode: exclusive, want: true},
		{name: "same file shared shared", left: "file:web/main.go", leftMode: shared, right: "file:web/main.go", rightMode: shared, want: false},
		{name: "different files", left: "file:web/main.go", leftMode: exclusive, right: "file:web/app.go", rightMode: exclusive, want: false},
		{name: "directory contains file", left: "dir:web", leftMode: exclusive, right: "file:web/src/main.go", rightMode: shared, want: true},
		{name: "file inside directory reverse", left: "file:web/src/main.go", leftMode: shared, right: "dir:web", rightMode: exclusive, want: true},
		{name: "directory equals file", left: "dir:web/main.go", leftMode: exclusive, right: "file:web/main.go", rightMode: exclusive, want: true},
		{name: "parent directory", left: "dir:web", leftMode: shared, right: "dir:web/src", rightMode: exclusive, want: true},
		{name: "child directory reverse", left: "dir:web/src", leftMode: exclusive, right: "dir:web", rightMode: shared, want: true},
		{name: "parent directory shared", left: "dir:web", leftMode: shared, right: "dir:web/src", rightMode: shared, want: false},
		{name: "directory boundary", left: "dir:web/src", leftMode: exclusive, right: "file:web/src2/main.go", rightMode: exclusive, want: false},
		{name: "file is not ancestor", left: "file:web", leftMode: exclusive, right: "file:web/src/main.go", rightMode: exclusive, want: false},
		{name: "file case fold", left: "file:Web/Main.go", leftMode: exclusive, right: "file:web/main.GO", rightMode: shared, want: true},
		{name: "directory ancestor case fold", left: "dir:Réports", leftMode: exclusive, right: "file:re\u0301ports/Final.md", rightMode: shared, want: true},
		{name: "directory boundary after case fold", left: "dir:WEB/src", leftMode: exclusive, right: "file:web/src2/main.go", rightMode: exclusive, want: false},
		{name: "unicode composed decomposed file", left: "file:résumé.md", leftMode: exclusive, right: "file:re\u0301sume\u0301.md", rightMode: shared, want: true},
		{name: "same semantic", left: "semantic:report", leftMode: exclusive, right: "semantic:report", rightMode: shared, want: true},
		{name: "same semantic shared", left: "semantic:report", leftMode: shared, right: "semantic:report", rightMode: shared, want: false},
		{name: "different semantic", left: "semantic:report", leftMode: exclusive, right: "semantic:appendix", rightMode: exclusive, want: false},
		{name: "semantic remains case sensitive", left: "semantic:Report", leftMode: exclusive, right: "semantic:report", rightMode: exclusive, want: false},
		{name: "semantic and path", left: "semantic:web", leftMode: exclusive, right: "dir:web", rightMode: exclusive, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := WorkOutputScopesConflict(
				WorkOutputScope{Scope: test.left, Mode: test.leftMode},
				WorkOutputScope{Scope: test.right, Mode: test.rightMode},
			)
			if err != nil {
				t.Fatalf("conflict: %v", err)
			}
			if got != test.want {
				t.Fatalf("conflict = %t, want %t", got, test.want)
			}
		})
	}
}

func TestWorkOutputScopesConflictRejectsInvalidOperands(t *testing.T) {
	tests := []struct {
		name  string
		left  WorkOutputScope
		right WorkOutputScope
	}{
		{
			name:  "invalid left grammar",
			left:  WorkOutputScope{Scope: "web/main.go"},
			right: WorkOutputScope{Scope: "file:web/main.go"},
		},
		{
			name:  "invalid right mode",
			left:  WorkOutputScope{Scope: "semantic:report"},
			right: WorkOutputScope{Scope: "semantic:report", Mode: "private"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := WorkOutputScopesConflict(test.left, test.right)
			if !errors.Is(err, ErrInvalidWorkOutputScope) {
				t.Fatalf("error = %v, want ErrInvalidWorkOutputScope", err)
			}
		})
	}
}
