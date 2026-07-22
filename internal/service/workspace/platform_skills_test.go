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
