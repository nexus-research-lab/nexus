package handlertest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type sqliteMigrationTemplate struct {
	once sync.Once
	data []byte
	err  error
}

const (
	testAppRootEnvName = "NEXUS_APP_ROOT"
	testAppRootGoMod   = "module example.invalid/nexus-test\n\ngo 1.26\n"
	testNexusctlMain   = "package main\n\nfunc main() {}\n"
)

var (
	sqliteMigrationTemplatesMu sync.Mutex
	sqliteMigrationTemplates   = map[string]*sqliteMigrationTemplate{}
)

// RunWithMinimalAppRoot 使用最小应用根运行不验证产品资源内容的服务测试。
//
// 平台 Skill 的完整发布行为由 workspace 专项测试覆盖；服务测试只需要
// 验证发布入口可用，避免每个隔离状态根重复复制真实产品 Skill 树。
func RunWithMinimalAppRoot(m *testing.M) int {
	return runWithAppRoot(m, createMinimalAppRoot)
}

// RunWithSelectedAppSkills 使用只包含指定 Skill 的应用根运行服务测试。
//
// 平台 Skill 发布专项测试仍验证真实文件内容，但不需要让无关的大体积资源参与
// 每个隔离状态根的指纹计算与复制。
func RunWithSelectedAppSkills(m *testing.M, skillNames ...string) int {
	return runWithAppRoot(m, func() (string, error) {
		return createAppRootWithSkills(skillNames)
	})
}

func runWithAppRoot(m *testing.M, createRoot func() (string, error)) int {
	if m == nil {
		_, _ = fmt.Fprintln(os.Stderr, "测试入口为空")
		return 1
	}
	root, err := createRoot()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "创建测试应用根失败: %v\n", err)
		return 1
	}
	previousRoot, hadPreviousRoot := os.LookupEnv(testAppRootEnvName)
	if err = os.Setenv(testAppRootEnvName, root); err != nil {
		_ = os.RemoveAll(root)
		_, _ = fmt.Fprintf(os.Stderr, "设置最小测试应用根失败: %v\n", err)
		return 1
	}

	exitCode := m.Run()
	if hadPreviousRoot {
		_ = os.Setenv(testAppRootEnvName, previousRoot)
	} else {
		_ = os.Unsetenv(testAppRootEnvName)
	}
	if err = os.RemoveAll(root); err != nil && exitCode == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "清理最小测试应用根失败: %v\n", err)
		return 1
	}
	return exitCode
}

func createAppRootWithSkills(skillNames []string) (string, error) {
	sourceRoot, err := sourceAppRoot()
	if err != nil {
		return "", err
	}
	root, err := createMinimalAppRoot()
	if err != nil {
		return "", err
	}
	cleanup := func(cause error) (string, error) {
		_ = os.RemoveAll(root)
		return "", cause
	}
	seen := make(map[string]struct{}, len(skillNames))
	for _, skillName := range skillNames {
		normalizedName := strings.TrimSpace(skillName)
		if normalizedName == "" ||
			normalizedName == "." ||
			filepath.Base(normalizedName) != normalizedName {
			return cleanup(fmt.Errorf("测试 Skill 名称非法: %q", skillName))
		}
		if _, exists := seen[normalizedName]; exists {
			continue
		}
		seen[normalizedName] = struct{}{}
		source := filepath.Join(sourceRoot, "skills", normalizedName)
		target := filepath.Join(root, "skills", normalizedName)
		if err = os.CopyFS(target, os.DirFS(source)); err != nil {
			return cleanup(fmt.Errorf("复制测试 Skill %q 失败: %w", normalizedName, err))
		}
	}
	return root, nil
}

func createMinimalAppRoot() (string, error) {
	root, err := os.MkdirTemp("", "nexus-test-app-root-")
	if err != nil {
		return "", err
	}
	cleanup := func(cause error) (string, error) {
		_ = os.RemoveAll(root)
		return "", cause
	}
	for _, directory := range []string{
		filepath.Join(root, "skills"),
		filepath.Join(root, "cmd", "nexusctl"),
		filepath.Join(root, "cmd", "nexuscfg"),
	} {
		if err = os.MkdirAll(directory, 0o755); err != nil {
			return cleanup(err)
		}
	}
	if err = os.WriteFile(filepath.Join(root, "go.mod"), []byte(testAppRootGoMod), 0o644); err != nil {
		return cleanup(err)
	}
	if err = os.WriteFile(
		filepath.Join(root, "cmd", "nexusctl", "main.go"),
		[]byte(testNexusctlMain),
		0o644,
	); err != nil {
		return cleanup(err)
	}
	if err = os.WriteFile(
		filepath.Join(root, "cmd", "nexuscfg", "main.go"),
		[]byte(testNexusctlMain),
		0o644,
	); err != nil {
		return cleanup(err)
	}
	return root, nil
}

func sourceAppRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("定位 Nexus 源码根失败")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("定位 Nexus 源码根失败: %w", err)
	}
	return root, nil
}

// NewConfig 返回HTTP 服务测试配置。
func NewConfig(t testing.TB) config.Config {
	t.Helper()

	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("HOME", root)
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18031,
		ProjectName:    "nexus-handler-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

// OpenSQLite 打开测试数据库。
func OpenSQLite(t testing.TB, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	return db
}

// CloseServer 在测试结束时释放 HTTP Server 自行持有的数据库与后台资源。
func CloseServer(t testing.TB, server interface{ Close(context.Context) error }) {
	t.Helper()
	t.Cleanup(func() {
		if err := server.Close(context.Background()); err != nil {
			t.Errorf("关闭 HTTP 服务失败: %v", err)
		}
	})
}

// MigrateSQLite 执行 SQLite migration。
func MigrateSQLite(t testing.TB, databaseURL string) {
	t.Helper()
	MigrateSQLiteFromDir(t, databaseURL, migrationDir(t))
}

// MigrateSQLiteFromDir 从预迁移快照复制 SQLite 测试数据库。
//
// 每个测试仍拿到独立数据库文件，但同一个测试进程只执行一次完整
// migration，避免几十个测试重复解析和执行同一批 schema 变更。
func MigrateSQLiteFromDir(t testing.TB, databaseURL string, migrationDirectory string) {
	t.Helper()

	if shouldUseSQLiteSnapshot(databaseURL) {
		template := sqliteMigrationTemplateFor(migrationDirectory)
		template.once.Do(func() {
			template.data, template.err = openMigratedSQLiteTemplate(migrationDirectory)
		})
		if template.err == nil {
			if err := cloneSQLiteTemplate(template.data, databaseURL); err == nil {
				return
			}
		}
	}

	migrateSQLiteDirect(t, databaseURL, migrationDirectory)
}

func sqliteMigrationTemplateFor(migrationDirectory string) *sqliteMigrationTemplate {
	key := normalizedMigrationDirectory(migrationDirectory)
	sqliteMigrationTemplatesMu.Lock()
	defer sqliteMigrationTemplatesMu.Unlock()
	template := sqliteMigrationTemplates[key]
	if template == nil {
		template = &sqliteMigrationTemplate{}
		sqliteMigrationTemplates[key] = template
	}
	return template
}

func openMigratedSQLiteTemplate(migrationDirectory string) ([]byte, error) {
	migrationDirectory = normalizedMigrationDirectory(migrationDirectory)
	digest := sha256.Sum256([]byte(migrationDirectory))
	dsn := fmt.Sprintf("file:nexus-test-migration-%x?mode=memory&cache=shared", digest[:8])
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		return nil, err
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}
	if err = goose.Up(db, migrationDirectory); err != nil {
		return nil, err
	}
	snapshotRoot, err := os.MkdirTemp("", "nexus-sqlite-template-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(snapshotRoot) }()
	snapshotPath := filepath.Join(snapshotRoot, "template.db")
	if _, err = db.Exec(`VACUUM INTO ?`, snapshotPath); err != nil {
		return nil, err
	}
	return os.ReadFile(snapshotPath)
}

func normalizedMigrationDirectory(migrationDirectory string) string {
	directory := filepath.Clean(strings.TrimSpace(migrationDirectory))
	absoluteDirectory, err := filepath.Abs(directory)
	if err != nil {
		return directory
	}
	return absoluteDirectory
}

func cloneSQLiteTemplate(template []byte, databaseURL string) error {
	if len(template) == 0 {
		return errors.New("SQLite migration template 为空")
	}
	if err := os.MkdirAll(filepath.Dir(databaseURL), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(databaseURL, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(template)
	closeErr := file.Close()
	if writeErr != nil || written != len(template) || closeErr != nil {
		_ = os.Remove(databaseURL)
		switch {
		case writeErr != nil:
			return writeErr
		case written != len(template):
			return fmt.Errorf("SQLite migration template 写入不完整: got=%d want=%d", written, len(template))
		default:
			return closeErr
		}
	}
	return nil
}

func shouldUseSQLiteSnapshot(databaseURL string) bool {
	value := strings.TrimSpace(databaseURL)
	return value != "" &&
		value != ":memory:" &&
		!strings.Contains(value, "?") &&
		!strings.HasPrefix(strings.ToLower(value), "file:")
}

func migrateSQLiteDirect(t testing.TB, databaseURL string, migrationDirectory string) {
	t.Helper()

	db := OpenSQLite(t, databaseURL)
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err := goose.Up(db, migrationDirectory); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func migrationDir(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
