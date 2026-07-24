package workspaceisolation

import (
	"context"
	"runtime"
	"testing"
)

func TestOwnerProcessReaperSkipsNonEnforceModes(t *testing.T) {
	for _, mode := range []Mode{ModeOff, ModeAudit} {
		reaper := OwnerProcessReaper{
			Mode:         mode,
			LauncherPath: "/does/not/exist",
		}
		if err := reaper.ReapOwnerProcesses(context.Background(), "owner-a"); err != nil {
			t.Fatalf("mode=%s should skip launcher: %v", mode, err)
		}
	}
}

func TestOwnerProcessReaperRejectsUnsafeOwner(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("owner cgroup 仅在 Linux enforce 生效")
	}
	reaper := OwnerProcessReaper{
		Mode:         ModeEnforce,
		LauncherPath: "/does/not/exist",
	}
	if err := reaper.ReapOwnerProcesses(context.Background(), "../owner"); err == nil {
		t.Fatal("不安全 owner 应被拒绝")
	}
}
