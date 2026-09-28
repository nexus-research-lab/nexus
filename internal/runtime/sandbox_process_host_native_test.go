//go:build darwin && cgo

// INPUT: 显式指定的真实 bootstrap helper、原生 launchd 与 SQLite Host 适配。
// OUTPUT: 真实任务只在持久放行后执行，后代回收后写入相同数据库终态。
// POS: 本机集成证据，不替代默认 client、App 路径保护或崩溃恢复验收。
package runtime

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSandboxProcessHostNativeLaunch(t *testing.T) {
	helper := os.Getenv("NEXUS_SUPERVISION_TEST_HELPER")
	if helper == "" {
		t.Skip("explicit native helper required")
	}
	data, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	h, store, _ := newProcessHostFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	p, err := supervision.Start(ctx, supervision.Config{HelperPath: helper, HelperSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Host: h}, supervision.Command{Version: 1, Command: "/bin/sh", Args: []string{"-c", "printf database-host-ok"}, Env: []string{"PATH=/usr/bin:/bin"}, Directory: "/"})
	if p != nil {
		defer p.Close(context.Background())
	}
	if err != nil {
		t.Fatal(err)
	}
	streams := p.Streams()
	defer streams.Stdout.Close()
	defer streams.Stderr.Close()
	output, err := io.ReadAll(streams.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "database-host-ok" {
		t.Fatalf("output=%q", output)
	}
	if _, err := p.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	s, found, err := store.LatestProcess(ctx, "owner", "session")
	if err != nil || !found || s.Phase != protocol.SandboxProcessReaped || s.Registration == nil || s.Evidence == nil || s.Intent.Key.LaunchID != p.Intent().ID {
		t.Fatalf("snapshot=%+v found=%v err=%v", s, found, err)
	}
	if _, err := os.Stat(filepath.Join(h.root.Name(), "p", p.Intent().ID)); !os.IsNotExist(err) {
		t.Fatal("launch files remain", err)
	}
}

func TestSandboxProcessSupervisorNativeStartup(t *testing.T) {
	helper := os.Getenv("NEXUS_SUPERVISION_TEST_HELPER")
	if helper == "" {
		t.Skip("explicit native helper required")
	}
	data, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []bridge.RuntimeKind{bridge.RuntimeClaude, bridge.RuntimeNXS} {
		t.Run(string(kind), func(t *testing.T) {
			h, store, _ := newProcessHostFixture(t)
			var captured bridge.Options
			manager := NewManagerWithFactory(runtimeFactoryFunc(func(o bridge.Options) Client { captured = o; return &fakeRuntimeClient{} }))
			manager.SetSandboxPolicyReceiptStore(store)
			if err := manager.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: helper, HelperSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}); err != nil {
				t.Fatal(err)
			}
			options := bridge.Options{Runtime: bridge.RuntimeOptions{Kind: kind}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}
			var lease *SandboxResourceLease
			expectedLeaseID := ""
			purposes := []supervision.Purpose{supervision.ClaudeSandboxProbe, supervision.ClaudeRestrictedProbe, supervision.VersionProbe, supervision.Runtime}
			if kind == bridge.RuntimeNXS {
				lease, err = Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()})
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Release()
				expectedLeaseID = lease.Marker().LeaseID
				options.Sandbox = &bridge.SandboxSettings{Resources: lease.Resources()}
				purposes = []supervision.Purpose{supervision.VersionProbe, supervision.Runtime}
			}
			startup, err := manager.BeginClientStartup(t.Context(), "session", "owner")
			if err != nil {
				t.Fatal(err)
			}
			defer startup.Close()
			if _, err := startup.GetOrCreateWithLease(t.Context(), options, nil, lease); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			for _, purpose := range purposes {
				cfg, err := captured.ProcessSupervision(ctx, purpose)
				if err != nil {
					t.Fatal(err)
				}
				p, err := supervision.Start(ctx, cfg, supervision.Command{Version: 1, Command: "/bin/sh", Args: []string{"-c", "printf supervisor-bound"}, Directory: "/", Env: []string{"PATH=/usr/bin:/bin"}})
				if p != nil {
					defer p.Close(context.Background())
				}
				if err != nil {
					t.Fatal(err)
				}
				streams := p.Streams()
				output, err := io.ReadAll(streams.Stdout)
				streams.Stdout.Close()
				streams.Stderr.Close()
				if err != nil || string(output) != "supervisor-bound" {
					t.Fatalf("%s output=%q err=%v", purpose, output, err)
				}
				if _, err := p.Wait(ctx); err != nil {
					t.Fatal(err)
				}
				got, found, err := store.LatestProcess(ctx, "owner", "session")
				if kind == bridge.RuntimeNXS && (got.Intent.Scratch.BaseIdentity == "" || got.Intent.Scratch.LeafIdentity == "" || filepath.Join(got.Intent.Scratch.BasePath, got.Intent.Scratch.LeafName) != lease.Path()) {
					t.Fatalf("missing trusted scratch binding: %+v", got.Intent.Scratch)
				}
				if err != nil || !found || got.Intent.Key.Generation != 1 || got.Intent.Purpose != protocol.SandboxProcessPurpose(purpose) || got.Phase != protocol.SandboxProcessReaped || got.Intent.LeaseID != expectedLeaseID {
					t.Fatalf("%s snapshot=%+v found=%v err=%v", purpose, got, found, err)
				}
			}
		})
	}
}
