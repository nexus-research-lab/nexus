package runtimebootstrap

import (
	"runtime/debug"
	"testing"
)

func TestPinnedBridgeVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		deps []*debug.Module
		want string
	}{
		{name: "missing"},
		{name: "unversioned", deps: []*debug.Module{{Path: bridgeModule}}},
		{name: "development", deps: []*debug.Module{{Path: bridgeModule, Version: "(devel)"}}},
		{name: "replacement", deps: []*debug.Module{{Path: bridgeModule, Version: "v0.1.34", Replace: &debug.Module{Path: "../bridge"}}}},
		{name: "pinned", deps: []*debug.Module{{Path: "other", Version: "v1.0.0"}, {Path: bridgeModule, Version: "v0.1.34"}}, want: "v0.1.34"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pinnedBridgeVersion(&debug.BuildInfo{Deps: tc.deps})
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("version=%q error=%v", got, err)
			}
		})
	}
}
