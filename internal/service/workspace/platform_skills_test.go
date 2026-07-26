// 平台 Skill 全局兼容根的同步回归测试。
package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestEnsurePlatformSkillLibrarySyncsNXSAndClaudeEntrypoints(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_CONFIG_DIR", configRoot)

	if err := EnsurePlatformSkillLibrary(); err != nil {
		t.Fatalf("同步平台 Skill 库失败: %v", err)
	}
	nxsSkill := filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "ima-skill", "SKILL.md")
	claudeSkill := filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "ima-skill", "SKILL.md")
	for _, path := range []string{nxsSkill, claudeSkill} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("平台 Skill 入口缺失 %s: %v", path, err)
		}
	}
	linkPath := filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills")
	if target, err := os.Readlink(linkPath); err == nil {
		if target != filepath.Join("..", ".agents", "skills") {
			t.Fatalf("Claude Skill 入口链接目标不正确: %q", target)
		}
	}
}

func TestEnsurePlatformSkillLibraryPublishesRuntimeReadableTree(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_CONFIG_DIR", configRoot)

	if err := EnsurePlatformSkillLibrary(); err != nil {
		t.Fatalf("同步平台 Skill 库失败: %v", err)
	}
	if err := filepath.Walk(appfs.PlatformSkillRoot(), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		permission := info.Mode().Perm()
		if info.IsDir() {
			if permission != 0o755 {
				t.Fatalf("平台 Skill 目录权限 = %o, want 755: %s", permission, path)
			}
			return nil
		}
		if permission != 0o644 && permission != 0o755 {
			t.Fatalf("平台 Skill 文件权限 = %o, want 644 or 755: %s", permission, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("遍历平台 Skill 根失败: %v", err)
	}
}

func TestReplacePlatformSkillLibraryCopiesReadOnlySource(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "skills")
	sourceSkill := filepath.Join(sourceRoot, "goal-manager")
	if err := os.MkdirAll(sourceSkill, 0o755); err != nil {
		t.Fatalf("创建测试 Skill 目录失败: %v", err)
	}
	sourceSkillFile := filepath.Join(sourceSkill, "SKILL.md")
	if err := os.WriteFile(sourceSkillFile, []byte("goal\n"), 0o644); err != nil {
		t.Fatalf("写入测试 Skill 文件失败: %v", err)
	}
	if err := os.Chmod(sourceSkillFile, 0o444); err != nil {
		t.Fatalf("收紧测试 Skill 文件权限失败: %v", err)
	}
	if err := os.Chmod(sourceSkill, 0o555); err != nil {
		t.Fatalf("收紧测试 Skill 目录权限失败: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(sourceSkill, 0o755)
		_ = os.Chmod(sourceSkillFile, 0o644)
	})

	targetRoot := filepath.Join(t.TempDir(), "platform-skills")
	if err := replacePlatformSkillLibrary(sourceRoot, targetRoot, "test-fingerprint"); err != nil {
		t.Fatalf("只读源 Skill 应可发布到暂存目录: %v", err)
	}
	publishedSkill := filepath.Join(targetRoot, ".agents", "skills", "goal-manager", "SKILL.md")
	content, err := os.ReadFile(publishedSkill)
	if err != nil {
		t.Fatalf("读取已发布 Skill 失败: %v", err)
	}
	if string(content) != "goal\n" {
		t.Fatalf("已发布 Skill 内容 = %q, want goal", content)
	}
}
