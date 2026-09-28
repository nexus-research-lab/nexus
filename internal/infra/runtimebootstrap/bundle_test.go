// INPUT: 显式真实 helper 与独立 App 目录夹具。
// OUTPUT: 固定版本/摘要/架构校验、链接/内容/路径篡改拒绝。
// POS: 随包加载合同，不替代完整 App 签名或 clean-host 验收。
package runtimebootstrap

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNativeBundleBootstrap(t *testing.T) {
	binary := os.Getenv("NEXUS_BOOTSTRAP_TEST_BINARY")
	if binary == "" {
		t.Skip("explicit native bootstrap binary required")
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	version := identity.Main.Version
	bundle := t.TempDir()
	contents := filepath.Join(bundle, "Contents")
	resources := filepath.Join(contents, "Resources")
	sidecar := filepath.Join(contents, "MacOS", "nexus-server")
	for _, dir := range []string{filepath.Dir(sidecar), filepath.Join(resources, "bin")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(sidecar, []byte("sidecar path fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(resources, helperRelative)
	if err := os.WriteFile(helper, data, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{Version: 1, RelativePath: helperRelative, BridgeVersion: version, Architecture: runtime.GOARCH, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	write := func(m Manifest) {
		t.Helper()
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resources, "runtime-bootstrap.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(manifest)
	canonicalHelper, err := filepath.EvalSymlinks(helper)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Load(sidecar, version); err != nil || got.Manifest != manifest || got.Path != canonicalHelper {
		t.Fatalf("valid bundle=%+v %v", got, err)
	}
	if _, err := Load(sidecar, "v0.0.1"); err == nil {
		t.Fatal("wrong pinned version accepted")
	}
	wrong := manifest
	wrong.RelativePath = "../outside"
	write(wrong)
	if _, err := Load(sidecar, version); err == nil {
		t.Fatal("manifest path escape accepted")
	}
	write(manifest)
	changed := append(append([]byte{}, data...), []byte("changed")...)
	if err := os.WriteFile(helper, changed, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(sidecar, version); err == nil {
		t.Fatal("modified binary accepted")
	}
	if err := os.Remove(helper); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(binary, helper); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(sidecar, version); err == nil {
		t.Fatal("helper link accepted")
	}
}
