package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestChangedPackageScope(t *testing.T) {
	root := t.TempDir()
	pkg := func(dir string, imports, tests, external []string) goPackage {
		return goPackage{Dir: filepath.Join(root, filepath.FromSlash(dir)), ImportPath: dir, Imports: imports, TestImports: tests, XTestImports: external}
	}
	graph := []goPackage{
		pkg("internal/domain", nil, nil, nil),
		pkg("internal/service", []string{"internal/domain"}, nil, nil),
		pkg("internal/handler", []string{"internal/service"}, nil, nil),
		pkg("internal/integration", nil, []string{"internal/domain"}, nil),
		pkg("internal/external", nil, nil, []string{"internal/domain"}),
		pkg("internal/unrelated", nil, nil, nil),
		pkg("internal/storage", nil, nil, nil),
		pkg("internal/storage/goal", nil, nil, nil),
		pkg("internal/app/server", nil, nil, nil),
		pkg("internal/migration", nil, nil, nil),
		pkg("cmd/nexus-server", nil, nil, nil),
		pkg("scripts/check-go-scope", nil, nil, nil),
	}
	for _, test := range []struct {
		name          string
		changed, want []string
	}{
		{"production change includes transitive and both test consumers", []string{"internal/domain/model.go"}, []string{"internal/domain", "internal/external", "internal/handler", "internal/integration", "internal/service"}},
		{"test-only change stays local", []string{"internal/domain/model_test.go"}, []string{"internal/domain"}},
		{"fixture change stays local", []string{"internal/domain/testdata/model.json"}, []string{"internal/domain"}},
		{"deleted file keeps its existing package consumers", []string{"internal/domain/deleted.go"}, []string{"internal/domain", "internal/external", "internal/handler", "internal/integration", "internal/service"}},
		{"migration selects actual database runners", []string{"db/migrations/postgres/00017.sql"}, []string{"cmd/nexus-server", "internal/app/server", "internal/migration", "internal/storage", "internal/storage/goal"}},
		{"removed package falls back safely", []string{"internal/removed/model.go"}, []string{"./..."}},
		{"dependency changes need full scope", []string{"go.mod"}, []string{"./..."}},
		{"tool changes test the selector", []string{"scripts/check-go-changed.sh"}, []string{"scripts/check-go-scope"}},
		{"web changes do not run Go tests", []string{"web/src/app.tsx"}, []string{}},
		{"mixed tests and production changes still expand", []string{"internal/domain/model_test.go", "internal/domain/model.go"}, []string{"internal/domain", "internal/external", "internal/handler", "internal/integration", "internal/service"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := selectPackages(root, test.changed, graph); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("scope = %v, want %v", got, test.want)
			}
		})
	}
}
