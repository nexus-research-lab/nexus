package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestEnsureUserSkillLibrarySharesNXSAndClaudeRoots(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(t.TempDir(), ".nexus"))
	if err := EnsureUserSkillLibrary("owner-a"); err != nil {
		t.Fatalf("创建用户级 Skill 源失败: %v", err)
	}
	sourcePath := filepath.Join(appfs.UserSkillDiscoveryRoot("owner-a"), "demo-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("创建用户级 Skill 目录失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("demo"), 0o644); err != nil {
		t.Fatalf("写入用户级 Skill 失败: %v", err)
	}
	claudePath := filepath.Join(appfs.UserSkillLibraryRoot("owner-a"), ".claude", "skills", "demo-skill", "SKILL.md")
	if payload, err := os.ReadFile(claudePath); err != nil {
		t.Fatalf("Claude 入口未共享用户级 Skill: %v", err)
	} else if string(payload) != "demo" {
		t.Fatalf("Claude 入口内容不正确: %q", payload)
	}
}

func TestMergeLegacyExternalSkillReferencesAndCleanup(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(t.TempDir(), ".nexus"))
	writeUserExternalSkill(t, "owner-a", "legacy-skill")
	workspacePath := t.TempDir()
	legacyPath := filepath.Join(workspacePath, ".agents", "skills", "legacy-skill")
	if err := os.MkdirAll(legacyPath, 0o755); err != nil {
		t.Fatalf("创建旧 workspace Skill 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyPath, "SKILL.md"), []byte("legacy"), 0o644); err != nil {
		t.Fatalf("写入旧 workspace Skill 失败: %v", err)
	}
	manifest := map[string]string{"name": "legacy-skill", "source_type": "external"}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("序列化旧 Skill manifest 失败: %v", err)
	}
	if err = os.WriteFile(filepath.Join(legacyPath, ".nexus-skill.json"), payload, 0o644); err != nil {
		t.Fatalf("写入旧 Skill manifest 失败: %v", err)
	}
	selected, changed, err := MergeLegacyExternalSkillReferences("owner-a", workspacePath, []string{"imagegen"})
	if err != nil {
		t.Fatalf("迁移旧 Skill 引用失败: %v", err)
	}
	if !changed || !slices.Equal(selected, []string{"imagegen", "external:legacy-skill"}) {
		t.Fatalf("迁移后的 Skill 引用 = %#v, changed=%v", selected, changed)
	}
	if err = EnsureExternalSkillWorkspaceClean("owner-a", workspacePath); err != nil {
		t.Fatalf("清理旧 workspace Skill 失败: %v", err)
	}
	if _, err = os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("旧 workspace Skill 未清理: %v", err)
	}
}

func TestMergeLegacyExternalSkillReferencesKeepsExistingReference(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(t.TempDir(), ".nexus"))
	selected, changed, err := MergeLegacyExternalSkillReferences(
		"owner-a",
		t.TempDir(),
		[]string{"imagegen", "external:existing-skill"},
	)
	if err != nil {
		t.Fatalf("规范化已有外部 Skill 引用失败: %v", err)
	}
	if changed || !slices.Equal(selected, []string{"imagegen", "external:existing-skill"}) {
		t.Fatalf("已有外部 Skill 引用被错误改写: %#v, changed=%v", selected, changed)
	}
}

func TestEnsureExternalSkillWorkspaceSkillCleanKeepsOtherLegacySkills(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(t.TempDir(), ".nexus"))
	writeUserExternalSkill(t, "owner-a", "remove-me")
	writeUserExternalSkill(t, "owner-a", "keep-me")
	workspacePath := t.TempDir()
	for _, name := range []string{"remove-me", "keep-me"} {
		skillPath := filepath.Join(workspacePath, ".agents", "skills", name)
		if err := os.MkdirAll(skillPath, 0o755); err != nil {
			t.Fatalf("创建旧 Skill %s 失败: %v", name, err)
		}
		payload := []byte(`{"name":"` + name + `","source_type":"external"}`)
		if err := os.WriteFile(filepath.Join(skillPath, ".nexus-skill.json"), payload, 0o644); err != nil {
			t.Fatalf("写入旧 Skill %s manifest 失败: %v", name, err)
		}
	}
	if err := EnsureExternalSkillWorkspaceSkillClean("owner-a", workspacePath, "remove-me"); err != nil {
		t.Fatalf("清理指定旧 Skill 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspacePath, ".agents", "skills", "remove-me")); !os.IsNotExist(err) {
		t.Fatalf("指定旧 Skill 未清理: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspacePath, ".agents", "skills", "keep-me")); err != nil {
		t.Fatalf("其他旧 Skill 不应被清理: %v", err)
	}
}

func TestEnsureExternalSkillWorkspaceSkillCleanRemovesLegacyEmptyMarker(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(t.TempDir(), ".nexus"))
	writeUserExternalSkill(t, "owner-a", "empty-marker")
	workspacePath := t.TempDir()
	emptyMarker := filepath.Join(workspacePath, ".agents", "skills", "empty-marker")
	if err := os.MkdirAll(emptyMarker, 0o755); err != nil {
		t.Fatalf("创建旧空目录标记失败: %v", err)
	}
	if err := EnsureExternalSkillWorkspaceSkillClean("owner-a", workspacePath, "empty-marker"); err != nil {
		t.Fatalf("清理旧空目录标记失败: %v", err)
	}
	if _, err := os.Stat(emptyMarker); !os.IsNotExist(err) {
		t.Fatalf("旧空目录标记未清理: %v", err)
	}
}

func TestMergeLegacyExternalSkillReferencesKeepsCopyWithoutUserSource(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(t.TempDir(), ".nexus"))
	workspacePath := t.TempDir()
	skillPath := filepath.Join(workspacePath, ".agents", "skills", "orphan-skill")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatalf("创建旧 Skill 副本失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, ".nexus-skill.json"), []byte(`{"name":"orphan-skill","source_type":"external"}`), 0o644); err != nil {
		t.Fatalf("写入旧 Skill manifest 失败: %v", err)
	}
	selected, changed, err := MergeLegacyExternalSkillReferences("owner-a", workspacePath, nil)
	if err != nil {
		t.Fatalf("检查孤立旧 Skill 失败: %v", err)
	}
	if changed || len(selected) != 0 {
		t.Fatalf("缺少用户级源时不应创建引用: %#v, changed=%v", selected, changed)
	}
	if err = EnsureExternalSkillWorkspaceClean("owner-a", workspacePath); err != nil {
		t.Fatalf("清理孤立旧 Skill 失败: %v", err)
	}
	if _, err = os.Stat(skillPath); err != nil {
		t.Fatalf("缺少用户级源时应保留旧副本: %v", err)
	}
}

func TestIsLegacyExternalSkillMarkerDoesNotClaimWorkspaceLocalSkill(t *testing.T) {
	workspacePath := t.TempDir()
	localPath := filepath.Join(workspacePath, ".agents", "skills", "local-skill")
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		t.Fatalf("创建 workspace-local Skill 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localPath, "SKILL.md"), []byte("local"), 0o644); err != nil {
		t.Fatalf("写入 workspace-local Skill 失败: %v", err)
	}
	if IsLegacyExternalSkillMarker(workspacePath, "local-skill") {
		t.Fatal("workspace-local Skill 不应被识别为旧外部部署")
	}
	directMarker := filepath.Join(workspacePath, ".agents", "direct-marker")
	if err := os.MkdirAll(directMarker, 0o755); err != nil {
		t.Fatalf("创建直属 workspace 目录失败: %v", err)
	}
	if IsLegacyExternalSkillMarker(workspacePath, "direct-marker") {
		t.Fatal(".agents 直属空目录不应被识别为旧外部部署")
	}
}

func writeUserExternalSkill(t *testing.T, ownerUserID string, skillName string) {
	t.Helper()
	if err := EnsureUserSkillLibrary(ownerUserID); err != nil {
		t.Fatalf("创建用户级 Skill 源失败: %v", err)
	}
	skillPath := filepath.Join(appfs.UserSkillDiscoveryRoot(ownerUserID), skillName)
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatalf("创建用户级 Skill %s 失败: %v", skillName, err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("test"), 0o644); err != nil {
		t.Fatalf("写入用户级 Skill %s 失败: %v", skillName, err)
	}
	payload := []byte(`{"name":"` + skillName + `","source_type":"external"}`)
	if err := os.WriteFile(filepath.Join(skillPath, ".nexus-skill.json"), payload, 0o644); err != nil {
		t.Fatalf("写入用户级 Skill %s manifest 失败: %v", skillName, err)
	}
}
