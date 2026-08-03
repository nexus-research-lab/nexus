package userroot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type workspaceRootFixture struct {
	config      config.Config
	databaseURL string
	db          *sql.DB
	oldRoot     string
	targetRoot  string
	records     []agentWorkspaceRecord
}

func TestScheduleDefersFileChangesUntilStartupMigratesAndSwitchesAgentPaths(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	manager := NewManager(fixture.config, fixture.db, logx.NewDiscardLogger())

	settings, err := manager.Schedule(t.Context(), fixture.targetRoot)
	if err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if !samePath(settings.WorkspacePath, fixture.oldRoot) ||
		settings.AppliedUsersPath != fixture.oldRoot ||
		settings.PendingUsersPath != fixture.targetRoot ||
		settings.MigratingUsersPath != "" {
		t.Fatalf("Schedule() settings = %+v", settings)
	}
	if _, err = os.Lstat(fixture.targetRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("设置保存不应在运行期创建目标 users 根: %v", err)
	}
	assertDatabaseWorkspaceRoot(t, fixture.db, fixture.records, fixture.oldRoot)

	systemMarker := filepath.Join(resolveAgentPathAt(fixture.oldRoot, fixture.records[0]), "marker.txt")
	deletedBeforeRestart := filepath.Join(resolveAgentPathAt(fixture.oldRoot, fixture.records[0]), "deleted-after-stage.txt")
	if err = os.Remove(deletedBeforeRestart); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(systemMarker, []byte("after-schedule\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = fixture.db.Close(); err != nil {
		t.Fatal(err)
	}

	startupConfig := fixture.config
	reconciled, err := ReconcileOnStartup(t.Context(), startupConfig, logx.NewDiscardLogger())
	if err != nil {
		t.Fatalf("ReconcileOnStartup() error = %v", err)
	}
	if !samePath(agentsvc.WorkspaceBasePath(reconciled), fixture.targetRoot) {
		t.Fatalf("reconciled workspace root = %q", agentsvc.WorkspaceBasePath(reconciled))
	}
	content, err := os.ReadFile(filepath.Join(resolveAgentPathAt(fixture.targetRoot, fixture.records[0]), "marker.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "after-schedule\n" {
		t.Fatalf("启动迁移未读取最终源数据: %q", content)
	}
	assertMigratedWorkspaceContent(t, fixture.targetRoot, "after-schedule")
	assertWorkspaceLinksMigrated(t, fixture, "after-schedule\n")
	assertFileContent(
		t,
		filepath.Join(fixture.targetRoot, "__system__", "runtime", "must-migrate.txt"),
		"runtime\n",
	)
	assertFileContent(
		t,
		filepath.Join(fixture.targetRoot, "__system__", "state", "rooms", "must-migrate.txt"),
		"state\n",
	)
	assertTranscriptProjectRebased(t, fixture)
	assertRoomWorkspacePathsRebased(t, fixture)
	if _, err = os.Lstat(filepath.Join(
		resolveAgentPathAt(fixture.targetRoot, fixture.records[0]),
		"deleted-after-stage.txt",
	)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("启动增量应同步 users 源端删除: %v", err)
	}

	db := openWorkspaceRootTestDB(t, fixture.databaseURL)
	defer db.Close()
	assertDatabaseWorkspaceRoot(t, db, fixture.records, fixture.targetRoot)
	stored, err := config.LoadRuntimeSettings()
	if err != nil {
		t.Fatal(err)
	}
	if stored.AppliedUsersPath != fixture.targetRoot {
		t.Fatalf("AppliedUsersPath = %q, want %q", stored.AppliedUsersPath, fixture.targetRoot)
	}
	if stored.PendingUsersPath != "" {
		t.Fatalf("PendingUsersPath = %q, want empty", stored.PendingUsersPath)
	}
	if stored.MigratingUsersPath != "" {
		t.Fatalf("MigratingUsersPath = %q, want empty", stored.MigratingUsersPath)
	}
	service := agentsvc.NewService(
		reconciled,
		agentrepo.NewSQLRepository(reconciled.DatabaseDriver, db),
	)
	if err = service.EnsureReady(context.Background()); err != nil {
		t.Fatalf("迁移后 Agent.EnsureReady() 不应再阻断启动: %v", err)
	}
	if _, err = os.Stat(systemMarker); err != nil {
		t.Fatalf("旧 workspace 应保留作备份: %v", err)
	}
}

func TestReconcileOnStartupRecoversLegacySettingWithoutAppliedRoot(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, true)
	if _, err := config.SaveRuntimeSettings(config.RuntimeSettings{
		WorkspacePath: fixture.targetRoot,
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}

	startupConfig := fixture.config
	reconciled, err := ReconcileOnStartup(t.Context(), startupConfig, logx.NewDiscardLogger())
	if err != nil {
		t.Fatalf("ReconcileOnStartup() error = %v", err)
	}
	if !samePath(agentsvc.WorkspaceBasePath(reconciled), fixture.targetRoot) {
		t.Fatalf("legacy setting 未切换到目标根: %q", agentsvc.WorkspaceBasePath(reconciled))
	}
	assertMigratedWorkspaceContent(t, fixture.targetRoot, "before-restart")
	db := openWorkspaceRootTestDB(t, fixture.databaseURL)
	defer db.Close()
	assertDatabaseWorkspaceRoot(t, db, fixture.records, fixture.targetRoot)
}

func TestReconcileTrustsCommittedAgentPathsWhenAppliedLedgerIsStale(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	manager := NewManager(fixture.config, fixture.db, logx.NewDiscardLogger())
	if _, err := manager.Schedule(t.Context(), fixture.targetRoot); err != nil {
		t.Fatal(err)
	}
	if err := applyTransition(
		t.Context(),
		fixture.db,
		fixture.records,
		fixture.oldRoot,
		fixture.targetRoot,
		fixture.config.DatabaseDriver,
	); err != nil {
		t.Fatal(err)
	}
	targetMarker := filepath.Join(resolveAgentPathAt(fixture.targetRoot, fixture.records[0]), "marker.txt")
	if err := os.WriteFile(targetMarker, []byte("target-is-live\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}

	startupConfig := fixture.config
	if _, err := ReconcileOnStartup(t.Context(), startupConfig, logx.NewDiscardLogger()); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, targetMarker, "target-is-live\n")
	settings, err := config.LoadRuntimeSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(settings.AppliedUsersPath, fixture.targetRoot) {
		t.Fatalf("stale applied users root = %q", settings.AppliedUsersPath)
	}
}

func TestScheduleRejectsNonEmptyUnmanagedTarget(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	if err := os.MkdirAll(fixture.targetRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.targetRoot, "unrelated.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(fixture.config, fixture.db, logx.NewDiscardLogger())

	if _, err := manager.Schedule(t.Context(), fixture.targetRoot); err == nil {
		t.Fatal("非空目标根不应被静默覆盖")
	}
	if _, err := os.Stat(config.RuntimeSettingsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("迁移失败不应写入新设置: %v", err)
	}
	assertDatabaseWorkspaceRoot(t, fixture.db, fixture.records, fixture.oldRoot)
}

func TestReconcileDoesNotOverwriteTargetPopulatedAfterSchedule(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	manager := NewManager(fixture.config, fixture.db, logx.NewDiscardLogger())
	if _, err := manager.Schedule(t.Context(), fixture.targetRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fixture.targetRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	unrelatedPath := filepath.Join(fixture.targetRoot, "unrelated.txt")
	if err := os.WriteFile(unrelatedPath, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}

	reconciled, err := ReconcileOnStartup(t.Context(), fixture.config, logx.NewDiscardLogger())
	if err != nil {
		t.Fatalf("目标被占用时仍应使用旧根启动: %v", err)
	}
	if !samePath(agentsvc.WorkspaceBasePath(reconciled), fixture.oldRoot) {
		t.Fatalf("fallback workspace root = %q, want %q", agentsvc.WorkspaceBasePath(reconciled), fixture.oldRoot)
	}
	assertFileContent(t, unrelatedPath, "keep\n")
	db := openWorkspaceRootTestDB(t, fixture.databaseURL)
	defer db.Close()
	assertDatabaseWorkspaceRoot(t, db, fixture.records, fixture.oldRoot)
	settings, err := config.LoadRuntimeSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(settings.AppliedUsersPath, fixture.oldRoot) ||
		!samePath(settings.PendingUsersPath, fixture.targetRoot) ||
		settings.MigratingUsersPath != "" {
		t.Fatalf("occupied target settings = %+v", settings)
	}
}

func TestReconcileResumesClaimedMigration(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	manager := NewManager(fixture.config, fixture.db, logx.NewDiscardLogger())
	settings, err := manager.Schedule(t.Context(), fixture.targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	settings.PendingUsersPath = ""
	settings.MigratingUsersPath = fixture.targetRoot
	if _, err = config.SaveRuntimeSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(fixture.targetRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	partialPath := filepath.Join(fixture.targetRoot, "partial-copy.txt")
	if err = os.WriteFile(partialPath, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = fixture.db.Close(); err != nil {
		t.Fatal(err)
	}

	reconciled, err := ReconcileOnStartup(t.Context(), fixture.config, logx.NewDiscardLogger())
	if err != nil {
		t.Fatalf("已认领的部分迁移应可续跑: %v", err)
	}
	if !samePath(agentsvc.WorkspaceBasePath(reconciled), fixture.targetRoot) {
		t.Fatalf("reconciled workspace root = %q, want %q", agentsvc.WorkspaceBasePath(reconciled), fixture.targetRoot)
	}
	if _, err = os.Lstat(partialPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("续跑应清理不属于源 users 树的部分文件: %v", err)
	}
	db := openWorkspaceRootTestDB(t, fixture.databaseURL)
	defer db.Close()
	assertDatabaseWorkspaceRoot(t, db, fixture.records, fixture.targetRoot)
	stored, err := config.LoadRuntimeSettings()
	if err != nil {
		t.Fatal(err)
	}
	if stored.PendingUsersPath != "" || stored.MigratingUsersPath != "" ||
		!samePath(stored.AppliedUsersPath, fixture.targetRoot) {
		t.Fatalf("resumed migration settings = %+v", stored)
	}
}

func TestCopyUsersTreeMirrorsCaseChangedEntryName(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	targetRoot := filepath.Join(root, "target")
	if err := os.MkdirAll(sourceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "room-state.json"), []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetRoot, "ROOM-STATE.JSON"), []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyUsersTree(t.Context(), sourceRoot, targetRoot); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, filepath.Join(targetRoot, "room-state.json"), "new\n")
	entries, err := os.ReadDir(targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "room-state.json" {
		t.Fatalf("目标目录未精确镜像大小写变更: %+v", entries)
	}
}

func TestScheduleRejectsUsersRootOverlappingHostAppData(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	targetRoot := filepath.Join(appfs.AppDir(), "moved-users")
	manager := NewManager(fixture.config, fixture.db, logx.NewDiscardLogger())

	if _, err := manager.Schedule(t.Context(), targetRoot); err == nil {
		t.Fatal("users 根不能放进宿主 app 数据目录")
	}
	if _, err := os.Stat(config.RuntimeSettingsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("拒绝重叠目标后不应保存设置: %v", err)
	}
	assertDatabaseWorkspaceRoot(t, fixture.db, fixture.records, fixture.oldRoot)
}

func TestScheduleRejectsCustomUsersRootWithEnforce(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, true)
	fixture.config.RuntimeIsolationMode = "enforce"
	manager := NewManager(fixture.config, fixture.db, logx.NewDiscardLogger())

	if _, err := manager.Schedule(t.Context(), fixture.targetRoot); err == nil {
		t.Fatal("enforce 的 root-owned users 根不能由应用设置改写")
	}
	if _, err := os.Stat(config.RuntimeSettingsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("拒绝自定义 users 根后不应保存设置: %v", err)
	}
	assertDatabaseWorkspaceRoot(t, fixture.db, fixture.records, fixture.oldRoot)
}

func TestReconcileFailureRestoresAppliedRootInsteadOfBlockingStartup(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	fixture.targetRoot = filepath.Join(fixture.oldRoot, "nested-target")
	if _, err := config.SaveRuntimeSettings(config.RuntimeSettings{
		WorkspacePath:    fixture.targetRoot,
		AppliedUsersPath: fixture.oldRoot,
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}

	startupConfig := fixture.config
	reconciled, err := ReconcileOnStartup(t.Context(), startupConfig, logx.NewDiscardLogger())
	if err != nil {
		t.Fatalf("迁移失败应回退旧根继续启动: %v", err)
	}
	if !samePath(agentsvc.WorkspaceBasePath(reconciled), fixture.oldRoot) {
		t.Fatalf("fallback workspace root = %q, want %q", agentsvc.WorkspaceBasePath(reconciled), fixture.oldRoot)
	}
	settings, err := config.LoadRuntimeSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(settings.WorkspacePath, fixture.oldRoot) ||
		!samePath(settings.AppliedUsersPath, fixture.oldRoot) ||
		settings.PendingUsersPath != "" {
		t.Fatalf("fallback settings = %+v", settings)
	}
	db := openWorkspaceRootTestDB(t, fixture.databaseURL)
	defer db.Close()
	assertDatabaseWorkspaceRoot(t, db, fixture.records, fixture.oldRoot)
}

func TestReconcileRejectsAppOverlapAndRecoversDefaultUsersRoot(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	invalidRoot := filepath.Join(appfs.AppDir(), "moved-users")
	if _, err := config.SaveRuntimeSettings(config.RuntimeSettings{
		WorkspacePath:    invalidRoot,
		AppliedUsersPath: fixture.oldRoot,
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}

	startupConfig := fixture.config
	startupConfig.WorkspacePath = invalidRoot
	reconciled, err := ReconcileOnStartup(t.Context(), startupConfig, logx.NewDiscardLogger())
	if err != nil {
		t.Fatalf("app 重叠目标应恢复默认 users 根: %v", err)
	}
	fallbackRoot := appfs.DefaultUsersRoot()
	if !samePath(agentsvc.WorkspaceBasePath(reconciled), fallbackRoot) {
		t.Fatalf("fallback users root = %q, want %q", agentsvc.WorkspaceBasePath(reconciled), fallbackRoot)
	}
	db := openWorkspaceRootTestDB(t, fixture.databaseURL)
	defer db.Close()
	assertDatabaseWorkspaceRoot(t, db, fixture.records, fallbackRoot)
	assertMigratedWorkspaceContent(t, fallbackRoot, "before-restart")
	settings, err := config.LoadRuntimeSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.WorkspacePath != "" ||
		!samePath(settings.AppliedUsersPath, fallbackRoot) ||
		settings.PendingUsersPath != "" {
		t.Fatalf("invalid target recovery settings = %+v", settings)
	}
}

func TestReconcileRecoversAlreadyAppliedCustomRootForEnforce(t *testing.T) {
	fixture := newWorkspaceRootFixture(t, false)
	if _, err := config.SaveRuntimeSettings(config.RuntimeSettings{
		WorkspacePath: fixture.oldRoot,
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Close(); err != nil {
		t.Fatal(err)
	}

	startupConfig := fixture.config
	startupConfig.RuntimeIsolationMode = "enforce"
	reconciled, err := ReconcileOnStartup(t.Context(), startupConfig, logx.NewDiscardLogger())
	if err != nil {
		t.Fatalf("enforce 应迁回 root-owned 默认 users 根: %v", err)
	}
	fallbackRoot := appfs.DefaultUsersRoot()
	if !samePath(agentsvc.WorkspaceBasePath(reconciled), fallbackRoot) {
		t.Fatalf("enforce fallback users root = %q, want %q", agentsvc.WorkspaceBasePath(reconciled), fallbackRoot)
	}
	db := openWorkspaceRootTestDB(t, fixture.databaseURL)
	defer db.Close()
	assertDatabaseWorkspaceRoot(t, db, fixture.records, fallbackRoot)
	assertMigratedWorkspaceContent(t, fallbackRoot, "before-restart")
}

func newWorkspaceRootFixture(t *testing.T, useDefaultRoot bool) *workspaceRootFixture {
	t.Helper()
	root := t.TempDir()
	stateRoot := filepath.Join(root, "state")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	t.Setenv(appfs.NexusUsersRootEnvName, "")
	oldRoot := filepath.Join(root, "workspace-old")
	if useDefaultRoot {
		oldRoot = appfs.UsersRoot()
	}
	targetRoot := filepath.Join(root, "workspace-new")
	databaseURL := filepath.Join(root, "nexus.db")
	db := openWorkspaceRootTestDB(t, databaseURL)
	migrateWorkspaceRootTestDB(t, db)
	fixture := &workspaceRootFixture{
		config: config.Config{
			AppMode:        "desktop",
			DatabaseDriver: "sqlite",
			DatabaseURL:    databaseURL,
			WorkspacePath:  oldRoot,
			DefaultAgentID: "nexus",
		},
		databaseURL: databaseURL,
		db:          db,
		oldRoot:     oldRoot,
		targetRoot:  targetRoot,
		records: []agentWorkspaceRecord{
			{agentID: "nexus", ownerUserID: "__system__"},
			{
				agentID:          "agent-a",
				ownerUserID:      "owner-a",
				workspaceDirName: "Amy",
			},
		},
	}
	for index := range fixture.records {
		record := &fixture.records[index]
		record.workspacePath = resolveAgentPathAt(oldRoot, *record)
		insertWorkspaceRootAgent(t, db, *record, index == 0)
		if err := os.MkdirAll(record.workspacePath, 0o770); err != nil {
			t.Fatal(err)
		}
		marker := "owner-a\n"
		if record.ownerUserID == "__system__" {
			marker = "before-restart\n"
		}
		if err := os.WriteFile(filepath.Join(record.workspacePath, "marker.txt"), []byte(marker), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(
		filepath.Join(resolveAgentPathAt(oldRoot, fixture.records[0]), "deleted-after-stage.txt"),
		[]byte("delete me\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	preferencesPath := filepath.Join(oldRoot, "__system__", "workspace", ".settings", "preferences.json")
	if err := os.MkdirAll(filepath.Dir(preferencesPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(preferencesPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	roomAssetPath := filepath.Join(oldRoot, "__system__", "workspace", ".rooms", "room-a", "asset.txt")
	if err := os.MkdirAll(filepath.Dir(roomAssetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(roomAssetPath, []byte("asset\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtimeMarker := filepath.Join(oldRoot, "__system__", "runtime", "must-migrate.txt")
	if err := os.MkdirAll(filepath.Dir(runtimeMarker), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeMarker, []byte("runtime\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldProjectRoot := filepath.Join(
		oldRoot,
		"__system__",
		"runtime",
		"projects",
		workspacestore.TranscriptProjectDirectoryName(resolveAgentPathAt(oldRoot, fixture.records[0])),
	)
	if err := os.MkdirAll(oldProjectRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(oldProjectRoot, "session-migrate.jsonl"),
		[]byte("{\"type\":\"user\"}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	stateMarker := filepath.Join(oldRoot, "__system__", "state", "rooms", "must-migrate.txt")
	if err := os.MkdirAll(filepath.Dir(stateMarker), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateMarker, []byte("state\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	systemWorkspace := resolveAgentPathAt(oldRoot, fixture.records[0])
	roomReference, err := json.Marshal(map[string]any{
		"workspace_path": systemWorkspace,
		"attachments": []any{map[string]any{
			"workspace_path": filepath.Join(systemWorkspace, "tmp", "attachment.png"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	roomReference = append(roomReference, '\n')
	if err = os.WriteFile(
		filepath.Join(filepath.Dir(stateMarker), "overlay.jsonl"),
		roomReference,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("marker.txt", filepath.Join(systemWorkspace, "marker-link")); err != nil {
		t.Logf("当前平台不能创建测试符号链接: %v", err)
	}
	if err := os.Link(
		filepath.Join(systemWorkspace, "marker.txt"),
		filepath.Join(systemWorkspace, "marker-hardlink"),
	); err != nil {
		t.Logf("当前平台不能创建测试硬链接: %v", err)
	}
	return fixture
}

func assertWorkspaceLinksMigrated(
	t *testing.T,
	fixture *workspaceRootFixture,
	wantHardlinkContent string,
) {
	t.Helper()
	sourceWorkspace := resolveAgentPathAt(fixture.oldRoot, fixture.records[0])
	targetWorkspace := resolveAgentPathAt(fixture.targetRoot, fixture.records[0])
	if _, err := os.Lstat(filepath.Join(sourceWorkspace, "marker-link")); err == nil {
		linkTarget, readErr := os.Readlink(filepath.Join(targetWorkspace, "marker-link"))
		if readErr != nil {
			t.Fatalf("workspace 符号链接未迁移: %v", readErr)
		}
		if linkTarget != "marker.txt" {
			t.Fatalf("workspace 符号链接目标 = %q", linkTarget)
		}
	}
	if _, err := os.Lstat(filepath.Join(sourceWorkspace, "marker-hardlink")); err == nil {
		assertFileContent(t, filepath.Join(targetWorkspace, "marker-hardlink"), wantHardlinkContent)
	}
}

func assertTranscriptProjectRebased(t *testing.T, fixture *workspaceRootFixture) {
	t.Helper()
	record := fixture.records[0]
	oldProjectName := workspacestore.TranscriptProjectDirectoryName(
		resolveAgentPathAt(fixture.oldRoot, record),
	)
	newProjectName := workspacestore.TranscriptProjectDirectoryName(
		resolveAgentPathAt(fixture.targetRoot, record),
	)
	projectsRoot := filepath.Join(fixture.targetRoot, "__system__", "runtime", "projects")
	assertFileContent(
		t,
		filepath.Join(projectsRoot, newProjectName, "session-migrate.jsonl"),
		"{\"type\":\"user\"}\n",
	)
	if oldProjectName != newProjectName {
		if _, err := os.Lstat(filepath.Join(projectsRoot, oldProjectName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("旧 transcript 项目目录应在迁移副本中完成重映射: %v", err)
		}
	}
}

func assertRoomWorkspacePathsRebased(t *testing.T, fixture *workspaceRootFixture) {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(
		fixture.targetRoot,
		"__system__",
		"state",
		"rooms",
		"overlay.jsonl",
	))
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		WorkspacePath string `json:"workspace_path"`
		Attachments   []struct {
			WorkspacePath string `json:"workspace_path"`
		} `json:"attachments"`
	}
	if err = json.Unmarshal(bytes.TrimSpace(payload), &row); err != nil {
		t.Fatal(err)
	}
	wantWorkspace := resolveAgentPathAt(fixture.targetRoot, fixture.records[0])
	if !samePath(row.WorkspacePath, wantWorkspace) {
		t.Fatalf("Room workspace path = %q, want %q", row.WorkspacePath, wantWorkspace)
	}
	wantAttachment := filepath.Join(wantWorkspace, "tmp", "attachment.png")
	if len(row.Attachments) != 1 || !samePath(row.Attachments[0].WorkspacePath, wantAttachment) {
		t.Fatalf("Room attachment workspace paths = %+v, want %q", row.Attachments, wantAttachment)
	}
}

func openWorkspaceRootTestDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func migrateWorkspaceRootTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位 workspace root 测试 migration")
	}
	migrationDirectory := filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
	if err := goose.Up(db, migrationDirectory); err != nil {
		t.Fatal(err)
	}
}

func insertWorkspaceRootAgent(t *testing.T, db *sql.DB, record agentWorkspaceRecord, main bool) {
	t.Helper()
	if _, err := db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES (?, ?, ?, '', '', 'active', ?, ?, ?)`,
		record.agentID,
		record.agentID,
		record.agentID,
		record.workspacePath,
		record.ownerUserID,
		main,
	); err != nil {
		t.Fatal(err)
	}
}

func assertMigratedWorkspaceContent(t *testing.T, root string, systemMarker string) {
	t.Helper()
	assertFileContent(t, filepath.Join(root, "__system__", "workspace", "nexus", "marker.txt"), systemMarker+"\n")
	assertFileContent(t, filepath.Join(root, "owner-a", "workspace", "Amy", "marker.txt"), "owner-a\n")
	assertFileContent(t, filepath.Join(root, "__system__", "workspace", ".settings", "preferences.json"), "{}\n")
	assertFileContent(t, filepath.Join(root, "__system__", "workspace", ".rooms", "room-a", "asset.txt"), "asset\n")
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s content = %q, want %q", path, content, want)
	}
}

func assertDatabaseWorkspaceRoot(
	t *testing.T,
	db *sql.DB,
	records []agentWorkspaceRecord,
	wantRoot string,
) {
	t.Helper()
	for _, record := range records {
		var workspacePath string
		if err := db.QueryRow(`SELECT workspace_path FROM agents WHERE id = ?`, record.agentID).Scan(&workspacePath); err != nil {
			t.Fatal(err)
		}
		want := resolveAgentPathAt(wantRoot, record)
		if !samePath(workspacePath, want) {
			t.Fatalf("Agent %s workspace = %q, want %q", record.agentID, workspacePath, want)
		}
	}
}
