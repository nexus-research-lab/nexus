// execution_and_queue_test.go 覆盖执行输出作用域、Goal 与输入队列协议测试。
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
