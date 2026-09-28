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
