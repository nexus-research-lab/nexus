package main

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"github.com/nexus-research-lab/nexus/internal/storage"

	"github.com/pressly/goose/v3"
)

func TestBuildRootCommandHelpDoesNotRunServer(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	buf := new(bytes.Buffer)
	cmd := buildRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	os.Args = []string{"nexus-server", "--help"}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected help to exit cleanly, got error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected help output, got empty string")
	}
	if bytes.Contains(buf.Bytes(), []byte("run goose up")) {
		t.Fatal("help output unexpectedly contains migration failure")
	}
	if bytes.Contains(buf.Bytes(), []byte("migrate")) {
		t.Fatal("help output should not expose a manual migrate subcommand")
	}
}

func TestEnsureOwnerFromEnvBootstrapsOwnerIdempotently(t *testing.T) {
	cfg := testServerConfig(t)
	logger := discardLogger()
	if err := runMigrations(cfg, logger); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}

	t.Setenv(authInitOwnerUsernameEnvName, "Admin")
	t.Setenv(authInitOwnerDisplayNameEnvName, "Root Admin")
	t.Setenv(authInitOwnerPasswordEnvName, "password123")
	if err := ensureOwnerFromEnv(context.Background(), cfg, logger); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}
	if err := ensureOwnerFromEnv(context.Background(), cfg, logger); err != nil {
		t.Fatalf("重复初始化 owner 应保持幂等: %v", err)
	}

	users := listAuthUsers(t, cfg)
	if len(users) != 1 {
		t.Fatalf("owner 初始化应只创建一个用户: %+v", users)
	}
	if users[0].Username != "admin" || users[0].DisplayName != "Root Admin" || users[0].Role != authsvc.RoleOwner {
		t.Fatalf("owner 用户不符合预期: %+v", users[0])
	}
}

func TestEnsureOwnerFromEnvRequiresPasswordWhenProfileProvided(t *testing.T) {
	cfg := testServerConfig(t)
	logger := discardLogger()
	if err := runMigrations(cfg, logger); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}

	t.Setenv(authInitOwnerUsernameEnvName, "admin")
	t.Setenv(authInitOwnerPasswordEnvName, "")
	err := ensureOwnerFromEnv(context.Background(), cfg, logger)
	if err == nil || !strings.Contains(err.Error(), authInitOwnerPasswordEnvName) {
		t.Fatalf("缺少密码时应返回明确错误: %v", err)
	}
}

func TestRunMigrationsRepairsLegacyAutomationPermissionVersionCollision(t *testing.T) {
	cfg := testServerConfig(t)
	db, migrationDir, err := openMigrationDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, migrationDir, 70); err != nil {
		t.Fatal(err)
	}
	applyLegacyAutomationPermissionSchema(t, db, migrationDir)
	if err = goose.UpTo(db, migrationDir, 85); err != nil {
		t.Fatalf("apply later migrations around legacy collision: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_permission_requests (
    request_id, owner_user_id, job_id, policy_revision, kind, status,
    tool_name, effect, input_fingerprint, capability_json
) VALUES (
    'request-legacy', 'owner-legacy', 'task-legacy', 1, 'tool', 'pending',
    'mcp__nexus_connectors__feishu_docx_read', 'read', 'fingerprint-legacy', '{}'
)
`); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	if err = runMigrations(cfg, discardLogger()); err != nil {
		t.Fatalf("repair legacy automation permission collision: %v", err)
	}
	verified, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer verified.Close()
	version, err := goose.GetDBVersion(verified)
	wantVersion := latestServerMigrationVersion(t, migrationDir)
	if err != nil || version != wantVersion {
		t.Fatalf("migration version after repair = %d, err=%v", version, err)
	}
	for _, migrationVersion := range []int64{71, 86} {
		var count int
		if err = verified.QueryRow(
			"SELECT COUNT(*) FROM goose_db_version WHERE version_id = ? AND is_applied = TRUE",
			migrationVersion,
		).Scan(&count); err != nil || count != 1 {
			t.Fatalf("migration %d marker count = %d, err=%v", migrationVersion, count, err)
		}
	}
	if !sqliteTestColumnExists(t, verified, "skill_sources", "managed_by") {
		t.Fatal("official private Skill migration 71 was not replayed")
	}
	var status string
	if err = verified.QueryRow(`
SELECT status
FROM automation_permission_requests
WHERE request_id = 'request-legacy'
`).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("legacy permission request changed: status=%q err=%v", status, err)
	}
}

func TestRunMigrationsRepairsShiftedRecoveryVersionCollision(t *testing.T) {
	cfg := testServerConfig(t)
	db, migrationDir, err := openMigrationDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, migrationDir, 120); err != nil {
		t.Fatal(err)
	}
	files := []string{
		"00122_automation_delivery_attempt_claim.sql",
		"00123_automation_task_deletion_claim.sql",
		"00124_automation_run_request_identity.sql",
		"00125_automation_heartbeat_wake_outbox.sql",
		"00126_agent_creation_requests.sql",
	}
	for index, name := range files {
		contents, readErr := os.ReadFile(filepath.Join(migrationDir, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		upSQL, _, found := strings.Cut(string(contents), "-- +goose Down")
		if !found {
			t.Fatalf("migration %s has no Goose Down boundary", name)
		}
		if _, err = db.Exec(upSQL); err != nil {
			t.Fatalf("apply shifted migration %s: %v", name, err)
		}
		if _, err = db.Exec(
			"INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, TRUE)",
			121+index,
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`
INSERT INTO agent_creation_requests (
    owner_user_id, creation_request_id, intent_digest, agent_id, workspace_path, status
) VALUES ('owner-legacy', 'web-create:legacy', 'digest', 'agent-legacy', '/tmp/agent-legacy', 'deleted')
`); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	if err = runMigrations(cfg, discardLogger()); err != nil {
		t.Fatalf("repair shifted recovery collision: %v", err)
	}
	verified, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer verified.Close()
	version, err := goose.GetDBVersion(verified)
	wantVersion := latestServerMigrationVersion(t, migrationDir)
	if err != nil || version != wantVersion {
		t.Fatalf("migration version after repair = %d, err=%v", version, err)
	}
	if !sqliteTestColumnExists(t, verified, "agents", "business_tags") {
		t.Fatal("official Agent business tags migration was not replayed")
	}
	var status string
	if err = verified.QueryRow(`
SELECT status
FROM agent_creation_requests
WHERE owner_user_id = 'owner-legacy' AND creation_request_id = 'web-create:legacy'
`).Scan(&status); err != nil || status != "deleted" {
		t.Fatalf("legacy Agent creation receipt changed: status=%q err=%v", status, err)
	}
	if err = verified.Close(); err != nil {
		t.Fatal(err)
	}
	if err = runMigrations(cfg, discardLogger()); err != nil {
		t.Fatalf("repeated startup after shifted recovery repair: %v", err)
	}
}

func latestServerMigrationVersion(t *testing.T, migrationDir string) int64 {
	t.Helper()
	migrations, err := goose.CollectMigrations(migrationDir, 0, math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) == 0 {
		t.Fatal("no server migrations found")
	}
	return migrations[len(migrations)-1].Version
}

func applyLegacyAutomationPermissionSchema(t *testing.T, db *sql.DB, migrationDir string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(migrationDir, "00086_automation_permission_pipeline.sql"))
	if err != nil {
		t.Fatal(err)
	}
	upSQL, _, found := strings.Cut(string(contents), "-- +goose Down")
	if !found {
		t.Fatal("automation permission migration has no Goose Down boundary")
	}
	if _, err = db.Exec(upSQL); err != nil {
		t.Fatalf("apply legacy automation permission schema: %v", err)
	}
	if _, err = db.Exec(
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES (71, TRUE)",
	); err != nil {
		t.Fatal(err)
	}
}

func sqliteTestColumnExists(t *testing.T, db *sql.DB, table string, column string) bool {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == column {
			return true
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}

func testServerConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "nexus.db"),
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func listAuthUsers(t *testing.T, cfg config.Config) []authsvc.User {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	service := authsvc.NewServiceWithDB(cfg, db)
	users, err := service.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	return users
}
