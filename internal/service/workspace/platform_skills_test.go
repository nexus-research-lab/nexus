// 平台 Skill 全局兼容根的同步回归测试。
package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestEnsurePlatformSkillLibrarySyncsNXSAndClaudeEntrypoints(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", configRoot)
	t.Setenv("NEXUS_CONFIG_DIR", configRoot)

	if err := EnsurePlatformSkillLibrary(); err != nil {
		t.Fatalf("同步平台 Skill 库失败: %v", err)
	}
	for _, path := range []string{
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "execution-orchestrator", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "execution-orchestrator", "references", "structure-selection.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "execution-orchestrator", "references", "graph-control.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "execution-orchestrator", "references", "responsibility-and-delivery.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "execution-orchestrator", "references", "recovery-and-alignment.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "execution-orchestrator", "references", "workgraph-distillation.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "execution-orchestrator", "references", "communication-and-continuity.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "goal-manager", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "goal-manager", "references", "create-and-retarget.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "goal-manager", "references", "complete-and-block.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "goal-manager", "references", "room-goals.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "execution-orchestrator", "references", "graph-control.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "execution-orchestrator", "references", "responsibility-and-delivery.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "execution-orchestrator", "references", "recovery-and-alignment.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "execution-orchestrator", "references", "workgraph-distillation.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "goal-manager", "references", "complete-and-block.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "visualize", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "visualize", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "automation", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "automation", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "docx", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "docx", "requirements.txt"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "docx", "scripts", "read_word.py"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "docx", "scripts", "read_word.py"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "navigation-and-starting.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "conversations-and-collaboration.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "agents-rooms-and-memory.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "goals-workgraphs-and-execution.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "proactive-followup-and-automation.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "browser-and-web-access.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "capabilities.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "settings-and-help.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "nexus-product-guide", "references", "current-feature-source-map.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "navigation-and-starting.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "conversations-and-collaboration.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "agents-rooms-and-memory.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "goals-workgraphs-and-execution.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "proactive-followup-and-automation.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "browser-and-web-access.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "capabilities.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "settings-and-help.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "nexus-product-guide", "references", "current-feature-source-map.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "ima-skill", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "ima-skill", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "pdf", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "pdf", "requirements.txt"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "pdf", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "pptx", "requirements.txt"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "pptx", "scripts", "read_presentation.py"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "pptx", "scripts", "read_presentation.py"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "xlsx", "scripts", "read_spreadsheet.py"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "xlsx", "requirements.txt"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "xlsx", "scripts", "read_spreadsheet.py"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "wechat-article-search", "SKILL.md"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "wechat-article-search", "requirements.txt"),
		filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "wechat-article-search", "scripts", "search.py"),
		filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills", "wechat-article-search", "scripts", "search.py"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("平台 Skill 入口缺失 %s: %v", path, err)
		}
	}
	graphControlPath := filepath.Join(
		appfs.PlatformSkillRoot(),
		".agents", "skills", "execution-orchestrator", "references", "graph-control.md",
	)
	graphControl, err := os.ReadFile(graphControlPath)
	if err != nil {
		t.Fatalf("读取已同步图控制契约失败: %v", err)
	}
	for _, exampleField := range []string{
		"nexus_plan",
		"operation: create",
		"logical_key",
		"subject",
		"objective",
		"deliverable",
		"acceptance_criteria",
		"output_scopes",
	} {
		if !strings.Contains(string(graphControl), exampleField) {
			t.Fatalf("已同步图控制指南缺少最小示例字段 %q", exampleField)
		}
	}
	for _, canonicalGuidance := range []string{
		"prepare_plan_execution",
		"parser-backed 描述",
		"Skill 不复制完整字段表",
		"不要根据单个报错逐字段删改",
	} {
		if !strings.Contains(string(graphControl), canonicalGuidance) {
			t.Fatalf("已同步图控制指南缺少真相源指引 %q", canonicalGuidance)
		}
	}
	linkPath := filepath.Join(appfs.PlatformSkillRoot(), ".claude", "skills")
	if target, err := os.Readlink(linkPath); err == nil {
		if target != filepath.Join("..", ".agents", "skills") {
			t.Fatalf("Claude Skill 入口链接目标不正确: %q", target)
		}
	}
	if runtime.GOOS == "windows" {
		return
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

func TestEnsurePlatformSkillLibraryRepairsUnreadableExistingTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows FileMode cannot represent runtime readability")
	}
	configRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", configRoot)
	t.Setenv("NEXUS_CONFIG_DIR", configRoot)

	if err := EnsurePlatformSkillLibrary(); err != nil {
		t.Fatalf("首次同步平台 Skill 库失败: %v", err)
	}
	fingerprint, err := skillLibraryFingerprint(filepath.Join(appfs.Root(), "skills"))
	if err != nil {
		t.Fatalf("计算平台 Skill 指纹失败: %v", err)
	}
	skillPath := filepath.Join(appfs.PlatformSkillRoot(), ".agents", "skills", "goal-manager", "SKILL.md")
	if err := os.Chmod(skillPath, 0o600); err != nil {
		t.Fatalf("收紧已发布 Skill 权限失败: %v", err)
	}
	if skillLibraryReady(appfs.PlatformSkillRoot(), fingerprint) {
		t.Fatal("不可供 runtime 读取的 Skill 树不应被判定为就绪")
	}
	if err := EnsurePlatformSkillLibrary(); err != nil {
		t.Fatalf("重新同步平台 Skill 库失败: %v", err)
	}
	info, err := os.Stat(skillPath)
	if err != nil {
		t.Fatalf("读取修复后的 Skill 失败: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("修复后的 Skill 权限 = %o, want 644", info.Mode().Perm())
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
	if err := replaceCompatibleSkillLibrary(sourceRoot, targetRoot, "test-fingerprint"); err != nil {
		t.Fatalf("只读源 Skill 应可发布到暂存目录: %v", err)
	}
	for _, publishedSkill := range []string{
		filepath.Join(targetRoot, ".agents", "skills", "goal-manager", "SKILL.md"),
		filepath.Join(targetRoot, ".claude", "skills", "goal-manager", "SKILL.md"),
	} {
		content, err := os.ReadFile(publishedSkill)
		if err != nil {
			t.Fatalf("读取已发布 Skill 失败 %q: %v", publishedSkill, err)
		}
		if string(content) != "goal\n" {
			t.Fatalf("已发布 Skill 内容 = %q, want goal: %s", content, publishedSkill)
		}
	}
}
