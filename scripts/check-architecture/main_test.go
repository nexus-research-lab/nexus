package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestDependencyBoundaries(t *testing.T) {
	for _, item := range []struct {
		from, to string
		reject   bool
	}{
		{"protocol", "config", true}, {"relay", "service/relay", true}, {"runtime", "protocol", false}, {"runtime", "infra/confinedfs", false}, {"runtime", "infra/authctx", true}, {"runtime", "infra/textutil", false}, {"infra/textutil", "protocol", true}, {"runtime", "service/goal", true},
		{"service/team", "handler/team", true}, {"service/team", "app", true}, {"service/team", "relay", false},
		{"storage/teamrelay", "service/relay", true}, {"storage/teamrelay", "relay", false},
		{"service/orchestration", "mcp/command", true}, {"service/orchestration/runtimehook", "mcp/command", false},
		{"app", "app/server", true}, {"app/server", "app", false}, {"message", "storage/workspace", true},
		{"infra/authctx", "service/auth", true},
	} {
		if got := forbidden(item.from, item.to); got != item.reject {
			t.Errorf("%s -> %s: reject=%v", item.from, item.to, got)
		}
	}
}

func TestCheckReadsEveryPackageAndIgnoresExternalImports(t *testing.T) {
	valid := `{"ImportPath":"` + internalPrefix + `service/team","Imports":["net/http","` + internalPrefix + `relay"]}`
	bad := `{"ImportPath":"` + internalPrefix + `storage/teamrelay","Imports":["` + internalPrefix + `service/relay"]}`
	if err := check(strings.NewReader(valid)); err != nil {
		t.Fatal(err)
	}
	if err := check(strings.NewReader(valid + bad)); err == nil {
		t.Fatal("第二个包的非法依赖未被拒绝")
	}
}

func TestSharedHelperShapeRejectsRenamedCopies(t *testing.T) {
	source := `package sample

import "strings"

func pickFirst(items ...string) string {
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			return item
		}
	}
	return ""
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func anyText(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

func skipsDot(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && trimmed != "." {
			return trimmed
		}
	}
	return ""
}

func keepsDomainRule(value any) string {
	typed, _ := value.(string)
	typed = strings.TrimSpace(typed)
	if typed == "" {
		return "default"
	}
	return typed
}
`
	file, err := parser.ParseFile(token.NewFileSet(), "sample.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"pickFirst": "FirstNonEmpty", "deref": "PointerValue", "anyText": "AnyString", "keepsDomainRule": "", "skipsDot": ""}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if got := sharedHelperShape(fn); got != want[fn.Name.Name] {
			t.Errorf("%s: got %q want %q", fn.Name.Name, got, want[fn.Name.Name])
		}
	}
}
