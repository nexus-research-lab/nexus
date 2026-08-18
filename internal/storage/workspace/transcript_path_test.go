package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTranscriptPathLookupHonorsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	history := NewAgentHistoryStore(t.TempDir())
	if _, err := history.resolveTranscriptPathContext(ctx, t.TempDir(), "session"); !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveTranscriptPathContext() error = %v, want context.Canceled", err)
	}

	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "git-dir"))
	t.Setenv("GIT_WORK_TREE", "")
	commandStarted := false
	paths, err := listTranscriptWorktreePathsContextWithCommand(
		ctx,
		t.TempDir(),
		func(commandCtx context.Context, name string, args ...string) *exec.Cmd {
			commandStarted = true
			return exec.CommandContext(commandCtx, name, args...)
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("listTranscriptWorktreePathsContext() error = %v, want context.Canceled", err)
	}
	if len(paths) != 0 {
		t.Fatalf("listTranscriptWorktreePathsContext() = %v, want no paths", paths)
	}
	if commandStarted {
		t.Fatal("预取消 context 不得启动 Git worktree lookup")
	}
}

func TestTranscriptWorktreeLookupRequired(t *testing.T) {
	t.Setenv("GIT_DIR", "")
	t.Setenv("GIT_WORK_TREE", "")

	t.Run("non repository", func(t *testing.T) {
		if transcriptWorktreeLookupRequired(t.TempDir()) {
			t.Fatal("非 Git workspace 不应启动 worktree 查询")
		}
	})

	t.Run("single worktree repository", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		nested := filepath.Join(root, "nested", "workspace")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		if transcriptWorktreeLookupRequired(nested) {
			t.Fatal("单 worktree 仓库不应启动 Git 子进程")
		}
	})

	t.Run("repository with linked worktrees", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(
			filepath.Join(root, ".git", "worktrees", "linked"),
			0o755,
		); err != nil {
			t.Fatal(err)
		}
		if !transcriptWorktreeLookupRequired(root) {
			t.Fatal("存在 linked worktree 时必须回退 Git 查询")
		}
	})

	t.Run("linked worktree metadata file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(root, ".git"),
			[]byte("gitdir: /tmp/example/.git/worktrees/linked\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		if !transcriptWorktreeLookupRequired(root) {
			t.Fatal("linked worktree 的 .git 文件必须触发 Git 查询")
		}
	})
}

func TestTranscriptWorktreeLookupRequiredHonorsGitEnvironment(t *testing.T) {
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "git-dir"))
	t.Setenv("GIT_WORK_TREE", "")
	if !transcriptWorktreeLookupRequired(t.TempDir()) {
		t.Fatal("显式 GIT_DIR 必须保留 Git 查询")
	}
}
