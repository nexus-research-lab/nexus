//go:build linux

package runtimeidentity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRuntimeIsolationHardLinksAllowsLinksWithinOneOwner(t *testing.T) {
	stateRoot := t.TempDir()
	ownerRoot := filepath.Join(stateRoot, "users", "owner-a")
	if err := os.MkdirAll(ownerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(ownerRoot, "source")
	if err := os.WriteFile(source, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, filepath.Join(ownerRoot, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeIsolationHardLinks(stateRoot); err != nil {
		t.Fatalf("同一 owner 内的完整硬链接集合不应被拒绝: %v", err)
	}
}

func TestValidateRuntimeIsolationHardLinksRejectsCrossBoundaryLinks(t *testing.T) {
	for _, test := range []struct {
		name       string
		targetPath func(string) string
	}{
		{
			name: "other owner",
			targetPath: func(stateRoot string) string {
				return filepath.Join(stateRoot, "users", "owner-b", "alias")
			},
		},
		{
			name: "outside protected roots",
			targetPath: func(stateRoot string) string {
				return filepath.Join(stateRoot, "outside", "alias")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			ownerRoot := filepath.Join(stateRoot, "users", "owner-a")
			if err := os.MkdirAll(ownerRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(ownerRoot, "source")
			if err := os.WriteFile(source, []byte("data"), 0o600); err != nil {
				t.Fatal(err)
			}
			target := test.targetPath(stateRoot)
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(source, target); err != nil {
				t.Fatal(err)
			}
			err := validateRuntimeIsolationHardLinks(stateRoot)
			if err == nil || !strings.Contains(err.Error(), "跨边界硬链接") {
				t.Fatalf("跨边界硬链接应被拒绝，得到: %v", err)
			}
		})
	}
}
