// initializer_and_skills_test.go 覆盖工作区初始化与平台/用户 Skill 投影测试。
package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRuntimeSkillSelectionSeparatesGlobalBindingsAndWorkspaceDisables(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	cfg := config.Config{WorkspacePath: filepath.Join(stateRoot, "workspace")}
	agentValue := protocol.Agent{
		AgentID:       "agent-1",
		OwnerUserID:   "owner-1",
		WorkspacePath: filepath.Join(UserSkillLibraryRoot(cfg, "owner-1"), "agent-1"),
		Options: protocol.Options{
			SkillIDs:         []string{"external:enabled-global"},
			DisabledSkillIDs: []string{"local-off", "goal-manager", "execution-orchestrator"},
		},
	}
	for _, path := range []string{
		filepath.Join(UserSkillDiscoveryRoot(cfg, agentValue.OwnerUserID), "enabled-global", "SKILL.md"),
		filepath.Join(UserSkillDiscoveryRoot(cfg, agentValue.OwnerUserID), "disabled-global", "SKILL.md"),
		filepath.Join(UserSkillDiscoveryRoot(cfg, agentValue.OwnerUserID), "same-name-local", "SKILL.md"),
		filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "local-on", "SKILL.md"),
		filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "local-off", "SKILL.md"),
		filepath.Join(agentValue.WorkspacePath, ".agents", "skills", "same-name-local", "SKILL.md"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("创建 Skill 目录失败: %v", err)
		}
		if err := os.WriteFile(path, []byte("# test\n"), 0o644); err != nil {
			t.Fatalf("写入 Skill 文件失败: %v", err)
		}
	}

	enabled, err := RuntimeSkillNamesForAgent(cfg, agentValue)
	if err != nil {
		t.Fatalf("读取 Agent 运行时 Skill 失败: %v", err)
	}
	for _, name := range []string{
		"enabled-global", "local-on", "goal-manager", "execution-orchestrator",
	} {
		if !slices.Contains(enabled, name) {
			t.Fatalf("运行时启用列表缺少 %q: %#v", name, enabled)
		}
	}
	if slices.Contains(enabled, "local-off") {
		t.Fatalf("显式停用的工作区 Skill 仍在启用列表: %#v", enabled)
	}

	disabled, err := RuntimeDisabledSkillNamesForAgent(cfg, agentValue)
	if err != nil {
		t.Fatalf("读取 Agent 运行时停用 Skill 失败: %v", err)
	}
	for _, name := range []string{"disabled-global", "local-off"} {
		if !slices.Contains(disabled, name) {
			t.Fatalf("运行时停用列表缺少 %q: %#v", name, disabled)
		}
	}
	if slices.Contains(disabled, "enabled-global") {
		t.Fatalf("已绑定的全局 Skill 被误判为停用: %#v", disabled)
	}
	if slices.Contains(disabled, "same-name-local") {
		t.Fatalf("动态发现的工作区同名 Skill 被误判为停用: %#v", disabled)
	}
	for _, name := range []string{"goal-manager", "execution-orchestrator"} {
		if slices.Contains(disabled, name) {
			t.Fatalf("受管 Skill %q 不能被陈旧持久化状态停用: %#v", name, disabled)
		}
	}
}

// EnsureInitialized 保证 workspace 模板就绪，并确保平台 Skill 不落入 Agent workspace。
func EnsureInitialized(
	agentID string,
	agentName string,
	workspacePath string,
	isMainAgent bool,
	createdAt time.Time,
) error {
	root := strings.TrimSpace(workspacePath)
	if root == "" {
		return fmt.Errorf("workspace_path 不能为空")
	}
	if err := os.MkdirAll(root, workspaceDirectoryMode()); err != nil {
		return err
	}
	rootFS, err := confinedfs.Open(root)
	if err != nil {
		return err
	}
	defer rootFS.Close()
	return EnsureInitializedAt(rootFS, agentID, agentName, isMainAgent, createdAt)
}

// EnsureInitializedAt 在已验证的 workspace 根中完成初始化。
func EnsureInitializedAt(
	rootFS *confinedfs.Root,
	agentID string,
	agentName string,
	isMainAgent bool,
	createdAt time.Time,
) error {
	return ensureInitializedAt(rootFS, agentID, agentName, isMainAgent, createdAt, nil)
}

// DeploySkill 把指定 skill 部署到目标 workspace。
func DeploySkill(skillName string, sourceDir string, workspacePath string, context map[string]string) error {
	if err := os.MkdirAll(workspacePath, workspaceDirectoryMode()); err != nil {
		return err
	}
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return DeploySkillAt(root, skillName, sourceDir, context)
}

// DeploySkillAt 把指定 skill 部署到已验证的 workspace 根。
func DeploySkillAt(
	root *confinedfs.Root,
	skillName string,
	sourceDir string,
	context map[string]string,
) error {
	sourceRoot, err := confinedfs.Open(sourceDir)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	return DeploySkillAtFromRoots(root, sourceRoot, skillName, context)
}

// DeploySkillAtFromRoots 在已固定的源与目标目录句柄之间部署 Skill。
//
// 外部 Skill 源可能位于另一个 owner 的共享目录；调用方应先通过
// owner-aware store 固定 sourceRoot，再把它传入，避免校验后重新打开
// record.SourcePath 形成 TOCTOU 或跨 owner 路径旁路。
func DeploySkillAtFromRoots(
	root *confinedfs.Root,
	sourceRoot *confinedfs.Root,
	skillName string,
	context map[string]string,
) error {
	if root == nil || sourceRoot == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	if err := validateWorkspaceSkillName(skillName); err != nil {
		return err
	}
	agentsSkillDir := filepath.ToSlash(filepath.Join(".agents", "skills", skillName))
	claudeSkillEntry := filepath.ToSlash(filepath.Join(".claude", "skills", skillName))
	if err := syncDirectoryRootAt(sourceRoot, root, agentsSkillDir, context); err != nil {
		return err
	}
	return ensureClaudeSkillEntryRootAt(
		sourceRoot,
		root,
		claudeSkillEntry,
		filepath.Join("..", "..", ".agents", "skills", skillName),
		context,
	)
}

// RuntimeSkillNames 合并 Agent 引用与 workspace-local Skill，形成运行时白名单。
//
// 外部引用以 external:<name> 形式持久化，进入 SDK 前还原为 canonical name；
// workspace-local Skill 仍从 workspace 文件发现，避免显式白名单把它过滤掉。
func RuntimeSkillNames(workspacePath string, selectedSkillIDs []string) ([]string, error) {
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return RuntimeSkillNamesAt(root, selectedSkillIDs, nil)
}

func syncDirectoryRootAt(
	sourceRoot *confinedfs.Root,
	root *confinedfs.Root,
	relativeTarget string,
	context map[string]string,
) error {
	if sourceRoot == nil || root == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	var err error
	if err = root.RemoveAll(relativeTarget); err != nil && !os.IsNotExist(err) {
		return err
	}
	targetRoot, err := root.OpenOrCreateRootNoSymlink(relativeTarget, workspaceDirectoryMode())
	if err != nil {
		return err
	}
	defer targetRoot.Close()
	return syncSkillDirectory(sourceRoot, targetRoot, context)
}

func syncSkillDirectory(
	sourceRoot *confinedfs.Root,
	targetRoot *confinedfs.Root,
	context map[string]string,
) error {
	entries, err := fs.ReadDir(sourceRoot.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, err := sourceRoot.Lstat(entry.Name())
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			sourceChild, err := sourceRoot.OpenRootNoSymlink(entry.Name())
			if err != nil {
				return err
			}
			targetChild, targetErr := targetRoot.OpenOrCreateRootNoSymlink(
				entry.Name(),
				workspaceDirectoryMode(),
			)
			if targetErr != nil {
				sourceChild.Close()
				return targetErr
			}
			copyErr := syncSkillDirectory(sourceChild, targetChild, context)
			sourceChild.Close()
			targetChild.Close()
			if copyErr != nil {
				return copyErr
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		sourceFile, err := sourceRoot.OpenFileNoSymlink(entry.Name(), os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		openedInfo, err := sourceFile.Stat()
		if err != nil {
			sourceFile.Close()
			return err
		}
		if entry.Name() == "SKILL.md" {
			content, readErr := io.ReadAll(sourceFile)
			sourceFile.Close()
			if readErr != nil {
				return readErr
			}
			rendered := renderTemplate(string(content), context)
			if err = targetRoot.WriteFileAtomic(
				entry.Name(),
				[]byte(strings.TrimSpace(rendered)+"\n"),
				workspaceFileMode(),
			); err != nil {
				return err
			}
			continue
		}
		err = targetRoot.WriteFileAtomicFrom(
			entry.Name(),
			sourceFile,
			workspaceCopyFileMode(openedInfo.Mode()),
		)
		sourceFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureClaudeSkillEntryRootAt(
	sourceRoot *confinedfs.Root,
	root *confinedfs.Root,
	entryPath string,
	relativeTarget string,
	context map[string]string,
) error {
	if sourceRoot == nil || root == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	err := ensureRelativeSymlinkAt(root, entryPath, relativeTarget)
	if err == nil {
		return nil
	}
	// Windows 默认可能没有目录 symlink 权限，失败时镜像一份给 Claude 读取。
	if mirrorErr := syncDirectoryRootAt(sourceRoot, root, entryPath, context); mirrorErr != nil {
		return fmt.Errorf("创建 Claude Skill 入口失败: %w；镜像目录也失败: %v", err, mirrorErr)
	}
	return nil
}

func TestCompatibleSkillLibrarySkipsMatchingPublicationAndRepairsDrift(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "skills")
	targetRoot := filepath.Join(t.TempDir(), "platform-skills")
	writeSkillSourceFile(t, sourceRoot, "demo/SKILL.md", "# demo\n", 0o600)
	writeSkillSourceFile(t, sourceRoot, "demo/scripts/run.sh", "#!/bin/sh\n", 0o700)
	if err := ensureCompatibleSkillLibrary(sourceRoot, targetRoot); err != nil {
		t.Fatalf("首次发布失败: %v", err)
	}

	// 发布会整体替换根目录；哨兵存活即证明内容一致时没有重发。
	sentinel := filepath.Join(targetRoot, "sentinel")
	if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureCompatibleSkillLibrary(sourceRoot, targetRoot); err != nil {
		t.Fatalf("重复发布失败: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("内容一致时不应重发: %v", err)
	}

	published := filepath.Join(targetRoot, ".agents", "skills", "demo", "SKILL.md")
	if err := os.Chmod(published, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(published, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureCompatibleSkillLibrary(sourceRoot, targetRoot); err != nil {
		t.Fatalf("修复发布失败: %v", err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("内容漂移后应整库重发: %v", err)
	}
	content, err := os.ReadFile(published)
	if err != nil || string(content) != "# demo\n" {
		t.Fatalf("重发后内容 = %q, err=%v", content, err)
	}
}

func TestCompatibleSkillLibraryRemovesOnlyStaleStaging(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "skills")
	parent := t.TempDir()
	targetRoot := filepath.Join(parent, "platform-skills")
	writeSkillSourceFile(t, sourceRoot, "demo/SKILL.md", "# demo\n", 0o600)
	stale := filepath.Join(parent, skillLibraryStagingPrefix+"stale")
	fresh := filepath.Join(parent, skillLibraryStagingPrefix+"fresh")
	writeSkillSourceFile(t, stale, "demo/SKILL.md", "old", 0o644)
	writeSkillSourceFile(t, fresh, "demo/SKILL.md", "new", 0o644)
	old := time.Now().Add(-2 * staleSkillLibraryStagingAge)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	if err := ensureCompatibleSkillLibrary(sourceRoot, targetRoot); err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("过期暂存目录应被回收: %v", err)
	}
	// 未过期的可能属于另一个正在发布的进程，必须保留。
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("未过期暂存目录不应删除: %v", err)
	}
}

func writeSkillSourceFile(t *testing.T, root string, relative string, content string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
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
