package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestEnsureUserSkillLibrarySharesNXSAndClaudeRoots(t *testing.T) {
	cfg := testSkillConfig(t)
	sourcePath := filepath.Join(UserSkillDiscoveryRoot(cfg, "owner-a"), "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("创建用户级 Skill 目录失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("demo"), 0o644); err != nil {
		t.Fatalf("写入用户级 Skill 失败: %v", err)
	}
	if err := EnsureUserSkillLibrary(cfg, "owner-a"); err != nil {
		t.Fatalf("创建用户级 Skill 源失败: %v", err)
	}
	claudePath := filepath.Join(UserSkillLibraryRoot(cfg, "owner-a"), ".claude", "skills", "demo-skill", "SKILL.md")
	if payload, err := os.ReadFile(claudePath); err != nil {
		t.Fatalf("Claude 入口未共享用户级 Skill: %v", err)
	} else if string(payload) != "demo" {
		t.Fatalf("Claude 入口内容不正确: %q", payload)
	}
}

func TestUserSkillRootsFollowAgentWorkspaceLayout(t *testing.T) {
	cfg := config.Config{WorkspacePath: filepath.Join(t.TempDir(), "workspace")}
	if got, want := UserSkillLibraryRoot(cfg, "owner-a"), filepath.Join(cfg.WorkspacePath, "owner-a", "workspace"); got != want {
		t.Fatalf("用户 Skill 根 = %q, want %q", got, want)
	}
	if got, want := UserSkillLibraryRoot(cfg, "__system__"), filepath.Join(cfg.WorkspacePath, "__system__", "workspace"); got != want {
		t.Fatalf("系统 Skill 根 = %q, want %q", got, want)
	}
	wantUserRoot := filepath.Join(cfg.WorkspacePath, "owner-a", "workspace")
	if got := SkillLibraryRoots(cfg, "owner-a"); len(got) != 2 || got[1] != wantUserRoot {
		t.Fatalf("runtime 用户 Skill 根 = %#v, want platform + %q", got, wantUserRoot)
	}
}

func TestSkillLibraryRootsSkipUnpreparedOptionalHostRoot(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	roots := SkillLibraryRoots(cfg, "owner-a")
	if len(roots) != 2 {
		t.Fatalf("未准备的可选宿主根不应传给 runtime: %#v", roots)
	}
}

func TestRefreshUserSkillLibraryUpdatesClaudeMirror(t *testing.T) {
	cfg := testSkillConfig(t)
	originalCreateSymlink := createSymlink
	createSymlink = func(string, string) error {
		return errors.New("symlink unavailable")
	}
	t.Cleanup(func() {
		createSymlink = originalCreateSymlink
	})

	if err := EnsureUserSkillLibrary(cfg, "owner-a"); err != nil {
		t.Fatalf("创建用户 Skill fallback 镜像失败: %v", err)
	}
	sourceRoot := UserSkillDiscoveryRoot(cfg, "owner-a")
	sourcePath := filepath.Join(sourceRoot, "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("创建用户 Skill 源失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("demo"), 0o644); err != nil {
		t.Fatalf("写入用户 Skill 源失败: %v", err)
	}
	claudePath := filepath.Join(UserSkillLibraryRoot(cfg, "owner-a"), ".claude", "skills", "demo-skill", "SKILL.md")
	if err := RefreshUserSkillLibrary(cfg, "owner-a"); err != nil {
		t.Fatalf("刷新用户 Skill fallback 镜像失败: %v", err)
	}
	if payload, err := os.ReadFile(claudePath); err != nil {
		t.Fatalf("读取 Claude Skill fallback 镜像失败: %v", err)
	} else if string(payload) != "demo" {
		t.Fatalf("Claude Skill fallback 镜像内容不正确: %q", payload)
	}

	if err := os.RemoveAll(filepath.Dir(sourcePath)); err != nil {
		t.Fatalf("删除用户 Skill 源失败: %v", err)
	}
	if err := RefreshUserSkillLibrary(cfg, "owner-a"); err != nil {
		t.Fatalf("删除后刷新 Claude Skill fallback 镜像失败: %v", err)
	}
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Fatalf("删除后的 Skill 仍残留在 Claude fallback 镜像: %v", err)
	}
}

func TestRefreshUserSkillLibraryRepairsDamagedClaudeMirror(t *testing.T) {
	cfg := testSkillConfig(t)
	originalCreateSymlink := createSymlink
	createSymlink = func(string, string) error {
		return errors.New("symlink unavailable")
	}
	t.Cleanup(func() {
		createSymlink = originalCreateSymlink
	})

	sourcePath := filepath.Join(UserSkillDiscoveryRoot(cfg, "owner-a"), "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("创建用户 Skill 源失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("demo"), 0o644); err != nil {
		t.Fatalf("写入用户 Skill 源失败: %v", err)
	}
	if err := EnsureUserSkillLibrary(cfg, "owner-a"); err != nil {
		t.Fatalf("创建用户 Skill fallback 镜像失败: %v", err)
	}

	claudePath := filepath.Join(UserSkillLibraryRoot(cfg, "owner-a"), ".claude", "skills", "demo-skill", "SKILL.md")
	if err := os.Remove(claudePath); err != nil {
		t.Fatalf("破坏 Claude Skill fallback 镜像失败: %v", err)
	}
	if err := EnsureUserSkillLibrary(cfg, "owner-a"); err != nil {
		t.Fatalf("检查已就绪的用户 Skill fallback 镜像失败: %v", err)
	}
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Fatalf("普通 readiness 检查不应递归扫描并修复镜像内容: %v", err)
	}
	if err := RefreshUserSkillLibrary(cfg, "owner-a"); err != nil {
		t.Fatalf("修复 Claude Skill fallback 镜像失败: %v", err)
	}
	if payload, err := os.ReadFile(claudePath); err != nil {
		t.Fatalf("读取修复后的 Claude Skill fallback 镜像失败: %v", err)
	} else if string(payload) != "demo" {
		t.Fatalf("修复后的 Claude Skill fallback 镜像内容不正确: %q", payload)
	}
}

func TestRefreshUserSkillLibraryKeepsLastGoodClaudeMirrorOnCopyFailure(t *testing.T) {
	cfg := testSkillConfig(t)
	originalCreateSymlink := createSymlink
	createSymlink = func(string, string) error {
		return errors.New("symlink unavailable")
	}
	t.Cleanup(func() {
		createSymlink = originalCreateSymlink
	})

	sourceRoot := filepath.Join(UserSkillDiscoveryRoot(cfg, "owner-a"), "demo-skill")
	sourcePath := filepath.Join(sourceRoot, "SKILL.md")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("创建用户 Skill 源失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("last-good"), 0o644); err != nil {
		t.Fatalf("写入用户 Skill 源失败: %v", err)
	}
	if err := EnsureUserSkillLibrary(cfg, "owner-a"); err != nil {
		t.Fatalf("创建用户 Skill fallback 镜像失败: %v", err)
	}
	if err := os.Symlink(filepath.Join(sourceRoot, "missing"), filepath.Join(sourceRoot, "broken-link")); err != nil {
		t.Skipf("当前平台无法创建测试符号链接: %v", err)
	}
	if err := RefreshUserSkillLibrary(cfg, "owner-a"); err == nil {
		t.Fatal("包含内部链接的用户 Skill 镜像刷新应失败")
	}

	claudePath := filepath.Join(
		UserSkillLibraryRoot(cfg, "owner-a"),
		".claude",
		"skills",
		"demo-skill",
		"SKILL.md",
	)
	if payload, err := os.ReadFile(claudePath); err != nil {
		t.Fatalf("刷新失败后用户 Skill last-good 镜像丢失: %v", err)
	} else if string(payload) != "last-good" {
		t.Fatalf("刷新失败后用户 Skill last-good 被改写: %q", payload)
	}
}

func testSkillConfig(t *testing.T) config.Config {
	t.Helper()
	configRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", configRoot)
	t.Setenv("NEXUS_CONFIG_DIR", configRoot)
	return config.Config{WorkspacePath: filepath.Join(configRoot, "workspace")}
}
