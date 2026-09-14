package main

import (
	"strings"
	"testing"
)

func TestDependencyBoundaries(t *testing.T) {
	for _, item := range []struct {
		from, to string
		reject   bool
	}{
		{"protocol", "config", true}, {"relay", "service/relay", true}, {"runtime", "protocol", false}, {"runtime", "service/goal", true},
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
