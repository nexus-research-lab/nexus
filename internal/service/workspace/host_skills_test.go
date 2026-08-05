package workspace

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func TestEnsureHostSkillLibraryPublishesStandardAgentsRoot(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourceRoot := filepath.Join(home, ".agents", "skills", "host-skill")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 源失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "SKILL.md"), []byte("host-v1"), 0o644); err != nil {
		t.Fatalf("写入宿主 Skill 源失败: %v", err)
	}

	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("同步宿主 Skill 兼容根失败: %v", err)
	}
	for _, path := range []string{
		filepath.Join(appfs.HostSkillRoot(), ".agents", "skills", "host-skill", "SKILL.md"),
		filepath.Join(appfs.HostSkillRoot(), ".claude", "skills", "host-skill", "SKILL.md"),
	} {
		if payload, err := os.ReadFile(path); err != nil {
			t.Fatalf("宿主 Skill 兼容入口缺失 %s: %v", path, err)
		} else if string(payload) != "host-v1" {
			t.Fatalf("宿主 Skill 兼容入口内容 = %q, want host-v1", payload)
		}
	}
	roots := SkillLibraryRoots(cfg, "owner-a")
	if len(roots) != 3 || roots[1] != appfs.HostSkillRoot() {
		t.Fatalf("桌面 runtime Skill 根 = %#v, want platform + host + owner", roots)
	}
	agentsRoot := filepath.Join(appfs.HostSkillRoot(), ".agents", "skills")
	rootBefore, err := os.Stat(agentsRoot)
	if err != nil {
		t.Fatalf("读取刷新前宿主 Skill 根失败: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceRoot, "SKILL.md"), []byte("host-v2"), 0o644); err != nil {
		t.Fatalf("更新宿主 Skill 源失败: %v", err)
	}
	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("刷新宿主 Skill 兼容根失败: %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(appfs.HostSkillRoot(), ".agents", "skills", "host-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("读取刷新后的宿主 Skill 失败: %v", err)
	}
	if string(payload) != "host-v2" {
		t.Fatalf("刷新后的宿主 Skill 内容 = %q, want host-v2", payload)
	}
	rootAfter, err := os.Stat(agentsRoot)
	if err != nil {
		t.Fatalf("读取刷新后宿主 Skill 根失败: %v", err)
	}
	if !os.SameFile(rootBefore, rootAfter) {
		t.Fatal("宿主 Skill 刷新不应替换稳定的 .agents/skills 根")
	}
	if err = os.RemoveAll(sourceRoot); err != nil {
		t.Fatalf("删除宿主 Skill 源失败: %v", err)
	}
	if err = EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("同步宿主 Skill 删除失败: %v", err)
	}
	if _, err = os.Stat(filepath.Join(agentsRoot, "host-skill")); !os.IsNotExist(err) {
		t.Fatalf("源中删除的宿主 Skill 仍留在投影: %v", err)
	}
	rootAfterDelete, err := os.Stat(agentsRoot)
	if err != nil || !os.SameFile(rootBefore, rootAfterDelete) {
		t.Fatalf("删除 Skill 不应替换稳定根: %v", err)
	}
}

func TestPrepareHostSkillLibraryPublishesEmptyStableRoot(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("创建宿主用户目录失败: %v", err)
	}

	if err := PrepareHostSkillLibrary(cfg); err != nil {
		t.Fatalf("准备宿主 Skill 稳定根失败: %v", err)
	}
	if info, err := os.Stat(filepath.Join(appfs.HostSkillRoot(), ".agents", "skills")); err != nil || !info.IsDir() {
		t.Fatalf("宿主 Skill 稳定根未就绪: %v, %v", info, err)
	}
	roots := SkillLibraryRoots(cfg, "owner-a")
	if len(roots) != 3 || roots[1] != appfs.HostSkillRoot() {
		t.Fatalf("空快照时 runtime Skill 根 = %#v, want platform + host + owner", roots)
	}
}

func TestPrepareHostSkillLibraryRepairsMalformedManagedLayout(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	hostRoot := appfs.HostSkillRoot()
	if err := os.MkdirAll(filepath.Join(hostRoot, ".agents"), 0o755); err != nil {
		t.Fatalf("创建宿主受管根失败: %v", err)
	}
	for _, path := range []string{hostRoot, filepath.Join(hostRoot, ".agents")} {
		if err := os.Chmod(path, 0o700); err != nil {
			t.Fatalf("收紧旧宿主受管目录权限失败: %v", err)
		}
	}
	for _, path := range []string{
		filepath.Join(hostRoot, ".agents", "skills"),
		filepath.Join(hostRoot, ".claude"),
	} {
		if err := os.WriteFile(path, []byte("malformed"), 0o644); err != nil {
			t.Fatalf("写入损坏的宿主 Skill 布局失败: %v", err)
		}
	}
	staleStaging := filepath.Join(hostRoot, ".host-skill-staging-stale", "orphan")
	if err := os.MkdirAll(staleStaging, 0o755); err != nil {
		t.Fatalf("创建遗留宿主 Skill staging 失败: %v", err)
	}

	if err := PrepareHostSkillLibrary(cfg); err != nil {
		t.Fatalf("修复宿主 Skill 受管布局失败: %v", err)
	}
	for _, path := range []string{
		hostRoot,
		filepath.Join(hostRoot, ".agents"),
		filepath.Join(hostRoot, ".agents", "skills"),
		filepath.Join(hostRoot, ".claude"),
		filepath.Join(hostRoot, ".claude", "skills"),
	} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("宿主 Skill 受管目录未修复 %s: %v, %v", path, info, err)
		} else if info.Mode().Perm()&0o555 != 0o555 {
			t.Fatalf("宿主 Skill 受管目录不可供 runtime 读取 %s: %v", path, info.Mode())
		}
	}
	if _, err := os.Stat(filepath.Dir(staleStaging)); err != nil {
		t.Fatalf("启动准备不应枚举并清理 staging: %v", err)
	}
	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("后台阶段清理遗留宿主 Skill staging 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(staleStaging)); !os.IsNotExist(err) {
		t.Fatalf("遗留宿主 Skill staging 未清理: %v", err)
	}
}

func TestPrepareHostSkillLibraryReplacesLinkedManagedRootWithoutFollowingIt(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 链接外部目标失败: %v", err)
	}
	sentinel := filepath.Join(target, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatalf("写入宿主 Skill 链接外部标记失败: %v", err)
	}
	hostRoot := appfs.HostSkillRoot()
	if err := os.MkdirAll(filepath.Dir(hostRoot), 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 受管父目录失败: %v", err)
	}
	if err := os.Symlink(target, hostRoot); err != nil {
		t.Skipf("当前平台无法创建测试符号链接: %v", err)
	}

	if err := PrepareHostSkillLibrary(cfg); err != nil {
		t.Fatalf("修复链接宿主 Skill 受管根失败: %v", err)
	}
	if info, err := os.Lstat(hostRoot); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("宿主 Skill 受管根未修复为真实目录: %v, %v", info, err)
	}
	if payload, err := os.ReadFile(sentinel); err != nil || string(payload) != "keep" {
		t.Fatalf("修复受管根时修改了链接外部目标: %q, %v", payload, err)
	}
}

func TestEnsureHostSkillLibraryRejectsProjectionAsSource(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	stateRoot := filepath.Dir(filepath.Dir(appfs.HostSkillRoot()))
	home := filepath.Dir(stateRoot)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := PrepareHostSkillLibrary(cfg); err != nil {
		t.Fatalf("准备宿主 Skill 受管根失败: %v", err)
	}
	sourceParent := filepath.Join(home, ".agents")
	if err := os.MkdirAll(sourceParent, 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 源父目录失败: %v", err)
	}
	createTestHostSkillDirectoryLink(
		t,
		filepath.Join(appfs.HostSkillRoot(), ".agents", "skills"),
		filepath.Join(sourceParent, "skills"),
	)

	if err := EnsureHostSkillLibrary(cfg); err == nil || !strings.Contains(err.Error(), "受管投影重叠") {
		t.Fatalf("宿主 Skill 源不应指回受管投影: %v", err)
	}
}

func TestEnsureHostSkillLibrarySnapshotsTopLevelDirectoryLink(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourceRoot := filepath.Join(home, ".agents", "skills")
	linkedSkill := filepath.Join(home, ".codex", "superpowers", "skills", "brainstorming")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 根失败: %v", err)
	}
	if err := os.MkdirAll(linkedSkill, 0o755); err != nil {
		t.Fatalf("创建链接目标 Skill 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(linkedSkill, "SKILL.md"), []byte("linked"), 0o644); err != nil {
		t.Fatalf("写入链接目标 Skill 失败: %v", err)
	}
	createTestHostSkillDirectoryLink(
		t,
		linkedSkill,
		filepath.Join(sourceRoot, "brainstorming"),
	)

	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("同步目录链接宿主 Skill 失败: %v", err)
	}
	publishedRoot := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"brainstorming",
	)
	payload, err := os.ReadFile(filepath.Join(publishedRoot, "SKILL.md"))
	if err != nil {
		t.Fatalf("读取目录链接 Skill 快照失败: %v", err)
	}
	if string(payload) != "linked" {
		t.Fatalf("目录链接 Skill 快照内容 = %q, want linked", payload)
	}
	info, err := os.Lstat(publishedRoot)
	if err != nil {
		t.Fatalf("读取目录链接 Skill 快照元数据失败: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("目录链接发布结果必须是普通目录: %v", info.Mode())
	}
	claudePath := filepath.Join(
		appfs.HostSkillRoot(),
		".claude",
		"skills",
		"brainstorming",
		"SKILL.md",
	)
	if payload, err = os.ReadFile(claudePath); err != nil {
		t.Fatalf("Claude 未发现目录链接 Skill: %v", err)
	} else if string(payload) != "linked" {
		t.Fatalf("Claude 目录链接 Skill 内容 = %q, want linked", payload)
	}
}

func TestEnsureHostSkillLibrarySkipsNestedCollection(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourceRoot := filepath.Join(home, ".agents", "skills")
	directPath := filepath.Join(sourceRoot, "brainstorming", "SKILL.md")
	nestedPath := filepath.Join(sourceRoot, "superpowers", "brainstorming", "SKILL.md")
	for path, content := range map[string]string{
		directPath: "direct",
		nestedPath: "nested",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("创建宿主 Skill 目录失败: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("写入宿主 Skill 失败: %v", err)
		}
	}
	legacyCollection := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"superpowers",
		"brainstorming",
		"SKILL.md",
	)
	if err := os.MkdirAll(filepath.Dir(legacyCollection), 0o755); err != nil {
		t.Fatalf("创建历史递归投影失败: %v", err)
	}
	if err := os.WriteFile(legacyCollection, []byte("legacy-nested"), 0o644); err != nil {
		t.Fatalf("写入历史递归投影失败: %v", err)
	}

	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("同步宿主 Skill 失败: %v", err)
	}
	claudePath := filepath.Join(
		appfs.HostSkillRoot(),
		".claude",
		"skills",
		"brainstorming",
		"SKILL.md",
	)
	payload, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("读取直接 Skill 投影失败: %v", err)
	}
	if string(payload) != "direct" {
		t.Fatalf("直接 Skill 投影 = %q, want direct", payload)
	}
	collectionProjection := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"superpowers",
	)
	if _, err = os.Stat(collectionProjection); !os.IsNotExist(err) {
		t.Fatalf("嵌套 Skill 集合不应进入 canonical 投影: %v", err)
	}
}

func TestEnsureHostSkillLibraryCopiesClaudeViewWhenLinksUnavailable(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourcePath := filepath.Join(
		home,
		".agents",
		"skills",
		"nested-skill",
		"SKILL.md",
	)
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("nested"), 0o644); err != nil {
		t.Fatalf("写入宿主 Skill 失败: %v", err)
	}
	originalCreateSymlink := createSymlink
	createSymlink = func(string, string) error {
		return errors.New("symlink unavailable")
	}
	t.Cleanup(func() {
		createSymlink = originalCreateSymlink
	})

	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("无 symlink 权限时同步宿主 Skill 失败: %v", err)
	}
	claudeSkill := filepath.Join(
		appfs.HostSkillRoot(),
		".claude",
		"skills",
		"nested-skill",
	)
	info, err := os.Lstat(claudeSkill)
	if err != nil {
		t.Fatalf("Claude fallback Skill 缺失: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("Claude fallback Skill 必须是普通目录: %v", info.Mode())
	}
	payload, err := os.ReadFile(filepath.Join(claudeSkill, "SKILL.md"))
	if err != nil || string(payload) != "nested" {
		t.Fatalf("Claude fallback Skill 内容 = %q, %v", payload, err)
	}
}

func TestEnsureHostSkillLibraryBoundsOneEntryWithoutBlockingPeers(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourceRoot := filepath.Join(home, ".agents", "skills")
	healthyPath := filepath.Join(sourceRoot, "healthy-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(healthyPath), 0o755); err != nil {
		t.Fatalf("创建正常 Skill 目录失败: %v", err)
	}
	if err := os.WriteFile(healthyPath, []byte("healthy"), 0o644); err != nil {
		t.Fatalf("写入正常 Skill 失败: %v", err)
	}
	deepRoot := filepath.Join(sourceRoot, "deep-skill")
	if err := os.MkdirAll(deepRoot, 0o755); err != nil {
		t.Fatalf("创建过深 Skill 目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deepRoot, "SKILL.md"), []byte("deep"), 0o644); err != nil {
		t.Fatalf("写入过深 Skill 清单失败: %v", err)
	}
	deepPath := deepRoot
	for index := 0; index <= hostSkillCopyMaxDepth; index++ {
		deepPath = filepath.Join(deepPath, "nested")
	}
	if err := os.MkdirAll(deepPath, 0o755); err != nil {
		t.Fatalf("创建超限 Skill 资源树失败: %v", err)
	}

	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("单个超限 Skill 不应阻断宿主库: %v", err)
	}
	if _, err := os.Stat(filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"deep-skill",
	)); !os.IsNotExist(err) {
		t.Fatalf("超限 Skill 不应进入投影: %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"healthy-skill",
		"SKILL.md",
	))
	if err != nil || string(payload) != "healthy" {
		t.Fatalf("正常同级 Skill 未继续发布: %q, %v", payload, err)
	}
}

func TestHostSkillBoundedReaderRejectsConcurrentGrowth(t *testing.T) {
	reader := &hostSkillBoundedReader{
		source:    strings.NewReader("1234"),
		remaining: 3,
	}
	if _, err := io.ReadAll(reader); !errors.Is(err, errHostSkillCopyLimit) {
		t.Fatalf("超过复制预算的动态增长未被拒绝: %v", err)
	}
	if reader.copied != 4 {
		t.Fatalf("复制计数 = %d, want 4", reader.copied)
	}
}

func TestReadBoundedHostSkillDirectoryStopsBeforeFullEnumeration(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"z-skill", "a-skill", "m-skill"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
			t.Fatalf("创建宿主 Skill 测试目录失败: %v", err)
		}
	}
	root, err := confinedfs.Open(directory)
	if err != nil {
		t.Fatalf("打开宿主 Skill 测试根失败: %v", err)
	}
	defer root.Close()

	if entries, exceeded, err := readBoundedHostSkillDirectory(root, 2); err != nil {
		t.Fatalf("有界枚举宿主 Skill 目录失败: %v", err)
	} else if !exceeded || entries != nil {
		t.Fatalf("超限目录枚举结果 = %#v, exceeded=%t", entries, exceeded)
	}
	entries, exceeded, err := readBoundedHostSkillDirectory(root, 3)
	if err != nil {
		t.Fatalf("枚举预算内宿主 Skill 目录失败: %v", err)
	}
	if exceeded {
		t.Fatal("预算内宿主 Skill 目录不应标记为超限")
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	want := []string{"a-skill", "m-skill", "z-skill"}
	if !slices.Equal(got, want) {
		t.Fatalf("宿主 Skill 目录排序 = %#v, want %#v", got, want)
	}
}

func TestValidateHostSkillEntryNameRejectsReservedNames(t *testing.T) {
	for _, name := range []string{"", " padded", "padded ", "external:demo", "EXTERNAL:demo"} {
		if err := validateHostSkillEntryName(name); err == nil {
			t.Fatalf("宿主 Skill 保留目录名应被拒绝: %q", name)
		}
	}
	for _, name := range []string{"demo-skill", "财务分析"} {
		if err := validateHostSkillEntryName(name); err != nil {
			t.Fatalf("有效宿主 Skill 目录名被拒绝 %q: %v", name, err)
		}
	}
}

func TestHostSkillSnapshotAccountsRejectedCopyWork(t *testing.T) {
	home := t.TempDir()
	sourcePath := filepath.Join(home, ".agents", "skills")
	skillPath := filepath.Join(sourcePath, "broken-skill")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 源失败: %v", err)
	}
	payload := []byte("already copied")
	for name, content := range map[string][]byte{
		"SKILL.md":    []byte("skill"),
		"payload.txt": payload,
	} {
		if err := os.WriteFile(filepath.Join(skillPath, name), content, 0o644); err != nil {
			t.Fatalf("写入宿主 Skill 文件失败: %v", err)
		}
	}
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(skillPath, "z-link")); err != nil {
		t.Skipf("当前平台无法创建测试符号链接: %v", err)
	}

	source, err := confinedfs.Open(sourcePath)
	if err != nil {
		t.Fatalf("打开宿主 Skill 源失败: %v", err)
	}
	defer source.Close()
	target, err := confinedfs.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开宿主 Skill staging 失败: %v", err)
	}
	defer target.Close()
	builder := hostSkillSnapshotBuilder{
		home:             home,
		sourcePath:       sourcePath,
		source:           source,
		watchDirectories: map[string]struct{}{},
	}
	if err = builder.copySourceEntry(target, "broken-skill"); err == nil {
		t.Fatal("包含内部链接的宿主 Skill 应被拒绝")
	}
	if builder.total.bytes < int64(len(payload)) || builder.total.entries < 3 {
		t.Fatalf("被拒绝 Skill 的复制成本未计入整轮预算: %+v", builder.total)
	}
}

func TestEnsureHostSkillLibraryKeepsLastGoodWhenHomeLookupFails(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourcePath := filepath.Join(home, ".agents", "skills", "host-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 源失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("last-good"), 0o644); err != nil {
		t.Fatalf("写入宿主 Skill 源失败: %v", err)
	}
	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("发布宿主 Skill 失败: %v", err)
	}

	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	if err := EnsureHostSkillLibrary(cfg); err == nil {
		t.Fatal("宿主 home 不可用时应报告刷新失败")
	}
	targetPath := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"host-skill",
		"SKILL.md",
	)
	if payload, err := os.ReadFile(targetPath); err != nil {
		t.Fatalf("宿主 home 读取失败后 last-good 丢失: %v", err)
	} else if string(payload) != "last-good" {
		t.Fatalf("宿主 home 读取失败后 last-good 被改写: %q", payload)
	}
}

func TestWatchHostSkillLibraryRefreshesChangedSource(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourcePath := filepath.Join(home, ".agents", "skills", "host-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("创建 watcher 测试 Skill 失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("v1"), 0o644); err != nil {
		t.Fatalf("写入 watcher 测试 Skill 失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	done := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go func() {
		watchHostSkillLibrary(ctx, cfg, logger, ready)
		close(done)
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("宿主 Skill watcher 未就绪")
	}
	if err := os.WriteFile(sourcePath, []byte("v2"), 0o644); err != nil {
		cancel()
		t.Fatalf("更新 watcher 测试 Skill 失败: %v", err)
	}

	targetPath := filepath.Join(
		appfs.HostSkillRoot(),
		".claude",
		"skills",
		"host-skill",
		"SKILL.md",
	)
	deadline := time.Now().Add(5 * time.Second)
	for {
		payload, err := os.ReadFile(targetPath)
		if err == nil && string(payload) == "v2" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("宿主 Skill watcher 未刷新投影: %q, %v", payload, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("宿主 Skill watcher 未随 context 停止")
	}
}

func TestWatchHostSkillLibraryDiscoversFirstCreatedSource(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("创建宿主用户目录失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	done := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go func() {
		watchHostSkillLibrary(ctx, cfg, logger, ready)
		close(done)
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("宿主 Skill watcher 未就绪")
	}

	sourcePath := filepath.Join(home, ".agents", "skills", "first-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		cancel()
		t.Fatalf("创建首个宿主 Skill 目录失败: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("first"), 0o644); err != nil {
		cancel()
		t.Fatalf("写入首个宿主 Skill 失败: %v", err)
	}

	targetPath := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"first-skill",
		"SKILL.md",
	)
	deadline := time.Now().Add(5 * time.Second)
	for {
		payload, err := os.ReadFile(targetPath)
		if err == nil && string(payload) == "first" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("宿主 Skill watcher 未发现首个源目录: %q, %v", payload, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("宿主 Skill watcher 未随 context 停止")
	}
}

func TestEnsureHostSkillLibrarySkipsEscapingLinkWithoutBlockingPeers(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourceRoot := filepath.Join(home, ".agents", "skills")
	healthyRoot := filepath.Join(sourceRoot, "healthy-skill")
	if err := os.MkdirAll(healthyRoot, 0o755); err != nil {
		t.Fatalf("创建正常宿主 Skill 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(healthyRoot, "SKILL.md"), []byte("healthy"), 0o644); err != nil {
		t.Fatalf("写入正常宿主 Skill 失败: %v", err)
	}
	escapingRoot := filepath.Join(t.TempDir(), "outside-skill")
	if err := os.MkdirAll(escapingRoot, 0o755); err != nil {
		t.Fatalf("创建越界链接目标失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(escapingRoot, "SKILL.md"), []byte("outside"), 0o644); err != nil {
		t.Fatalf("写入越界链接目标失败: %v", err)
	}
	createTestHostSkillDirectoryLink(
		t,
		escapingRoot,
		filepath.Join(sourceRoot, "escaping-skill"),
	)

	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("单个越界 Skill 不应阻断宿主库同步: %v", err)
	}
	healthyPath := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"healthy-skill",
		"SKILL.md",
	)
	if payload, err := os.ReadFile(healthyPath); err != nil {
		t.Fatalf("正常宿主 Skill 未继续发布: %v", err)
	} else if string(payload) != "healthy" {
		t.Fatalf("正常宿主 Skill 内容 = %q, want healthy", payload)
	}
	escapingPath := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"escaping-skill",
	)
	if _, err := os.Stat(escapingPath); !os.IsNotExist(err) {
		t.Fatalf("越界宿主 Skill 不应进入安全快照: %v", err)
	}
}

func TestEnsureHostSkillLibraryKeepsLastGoodProjectionOnInvalidSource(t *testing.T) {
	cfg := testSkillConfig(t)
	cfg.AppMode = "desktop"
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sourceRoot := filepath.Join(home, ".agents", "skills", "host-skill")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("创建宿主 Skill 源失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "SKILL.md"), []byte("host-v1"), 0o644); err != nil {
		t.Fatalf("写入宿主 Skill 源失败: %v", err)
	}
	healthyRoot := filepath.Join(home, ".agents", "skills", "healthy-skill")
	if err := os.MkdirAll(healthyRoot, 0o755); err != nil {
		t.Fatalf("创建正常宿主 Skill 源失败: %v", err)
	}
	healthyFile := filepath.Join(healthyRoot, "SKILL.md")
	if err := os.WriteFile(healthyFile, []byte("healthy-v1"), 0o644); err != nil {
		t.Fatalf("写入正常宿主 Skill 源失败: %v", err)
	}
	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("首次同步宿主 Skill 失败: %v", err)
	}

	brokenLink := filepath.Join(sourceRoot, "broken-link")
	if err := os.Symlink(filepath.Join(home, "outside"), brokenLink); err != nil {
		t.Skipf("当前平台无法创建测试符号链接: %v", err)
	}
	if err := os.WriteFile(healthyFile, []byte("healthy-v2"), 0o644); err != nil {
		t.Fatalf("更新正常宿主 Skill 源失败: %v", err)
	}
	if err := EnsureHostSkillLibrary(cfg); err != nil {
		t.Fatalf("单个非法宿主 Skill 不应阻断其他 Skill: %v", err)
	}
	targetPath := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"host-skill",
		"SKILL.md",
	)
	if payload, err := os.ReadFile(targetPath); err != nil {
		t.Fatalf("读取 last-good 宿主 Skill 投影失败: %v", err)
	} else if string(payload) != "host-v1" {
		t.Fatalf("非法刷新覆盖了 last-good 宿主 Skill 投影: %q", payload)
	}
	healthyTarget := filepath.Join(
		appfs.HostSkillRoot(),
		".agents",
		"skills",
		"healthy-skill",
		"SKILL.md",
	)
	if payload, err := os.ReadFile(healthyTarget); err != nil {
		t.Fatalf("读取继续刷新的正常宿主 Skill 失败: %v", err)
	} else if string(payload) != "healthy-v2" {
		t.Fatalf("正常宿主 Skill 未独立刷新: %q", payload)
	}
}
