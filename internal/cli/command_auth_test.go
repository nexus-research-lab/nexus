package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestAuthAndUserCommands(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	statusPayload := runCLICommand(t, cfg, "auth", "status")
	statusItem := asMap(t, statusPayload["item"])
	if !asBool(t, statusItem["setup_required"]) {
		t.Fatalf("初始 auth status 应要求 setup: %+v", statusItem)
	}

	initPayload := runCLICommand(
		t,
		cfg,
		"auth",
		"init-owner",
		"--username",
		"admin",
		"--display-name",
		"系统管理员",
		"--password",
		"password123",
	)
	initItem := asMap(t, initPayload["item"])
	if initItem["username"] != "admin" {
		t.Fatalf("init-owner 返回数据不正确: %+v", initItem)
	}

	listPayload := runCLICommand(t, cfg, "user", "list")
	items, ok := listPayload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("user list 结果不正确: %+v", listPayload)
	}

	resetPayload := runCLICommand(
		t,
		cfg,
		"user",
		"reset-password",
		"--username",
		"admin",
		"--password",
		"password456",
	)
	resetItem := asMap(t, resetPayload["item"])
	if resetItem["username"] != "admin" {
		t.Fatalf("reset-password 返回数据不正确: %+v", resetItem)
	}
}

func TestInitOwnerCommandSupportsPasswordStdin(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	initPayload := runCLICommandWithEnvAndInput(
		t,
		cfg,
		nil,
		"password123\n",
		"auth",
		"init-owner",
		"--username",
		"admin",
		"--password-stdin",
	)
	initItem := asMap(t, initPayload["item"])
	if initItem["username"] != "admin" {
		t.Fatalf("init-owner(password-stdin) 返回数据不正确: %+v", initItem)
	}
}

func TestScopedAgentCommands(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	ownerPayload := runCLICommand(
		t,
		cfg,
		"auth",
		"init-owner",
		"--username",
		"admin",
		"--password",
		"password123",
	)
	ownerID := asString(t, asMap(t, ownerPayload["item"])["user_id"])

	alicePayload := runCLICommand(
		t,
		cfg,
		"user",
		"create",
		"--username",
		"alice",
		"--password",
		"password123",
	)
	aliceID := asString(t, asMap(t, alicePayload["item"])["user_id"])

	runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: ownerID},
		"agent",
		"create",
		"--name",
		"owner-helper",
	)
	runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: aliceID},
		"agent",
		"create",
		"--name",
		"alice-helper",
	)

	ownerList := runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: ownerID},
		"agent",
		"list",
	)
	assertAgentNames(t, ownerList["items"], "nexus", "owner-helper")
	assertAgentNamesAbsent(t, ownerList["items"], "alice-helper")

	aliceList := runCLICommandWithEnv(
		t,
		cfg,
		map[string]string{nexusctlUserIDEnvName: aliceID},
		"agent",
		"list",
	)
	assertAgentNames(t, aliceList["items"], "nexus", "alice-helper")
	assertAgentNamesAbsent(t, aliceList["items"], "owner-helper")

	errText := runCLICommandError(t, cfg, nil, "agent", "list")
	if !strings.Contains(errText, "必须显式提供 --scope-user-id") {
		t.Fatalf("未返回多用户作用域错误: %s", errText)
	}
}

func runCLICommand(t *testing.T, cfg config.Config, args ...string) map[string]any {
	return runCLICommandWithEnv(t, cfg, nil, args...)
}

func runCLICommandWithEnv(
	t *testing.T,
	cfg config.Config,
	env map[string]string,
	args ...string,
) map[string]any {
	return runCLICommandWithEnvAndInput(t, cfg, env, "", args...)
}

func runCLICommandWithEnvAndInput(
	t *testing.T,
	cfg config.Config,
	env map[string]string,
	stdin string,
	args ...string,
) map[string]any {
	t.Helper()

	payload, executeErr, output := executeCLICommandWithInput(t, cfg, env, stdin, args...)
	if executeErr != nil {
		t.Fatalf("执行 CLI 命令失败: %v, output=%s", executeErr, output)
	}
	return payload
}

func runCLICommandError(
	t *testing.T,
	cfg config.Config,
	env map[string]string,
	args ...string,
) string {
	t.Helper()

	_, executeErr, output := executeCLICommandWithInput(t, cfg, env, "", args...)
	if executeErr == nil {
		t.Fatalf("命令预期失败但成功了: %s", strings.Join(args, " "))
	}
	if output != "" {
		return output
	}
	return executeErr.Error()
}

func executeCLICommandWithInput(
	t *testing.T,
	cfg config.Config,
	env map[string]string,
	stdin string,
	args ...string,
) (map[string]any, error, string) {
	t.Helper()

	for _, key := range []string{nexusctlUserIDEnvName} {
		value, ok := env[key]
		if ok {
			t.Setenv(key, value)
			continue
		}
		t.Setenv(key, "")
	}

	command, err := New(cfg)
	if err != nil {
		t.Fatalf("创建 CLI 命令失败: %v", err)
	}
	command.SetArgs(args)
	command.SetIn(strings.NewReader(stdin))

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建 stdout 管道失败: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	executeErr := command.Execute()
	_ = writer.Close()

	var buffer bytes.Buffer
	if _, err = buffer.ReadFrom(reader); err != nil {
		t.Fatalf("读取 CLI 输出失败: %v", err)
	}
	_ = reader.Close()

	output := strings.TrimSpace(buffer.String())
	if executeErr != nil {
		return nil, executeErr, output
	}
	var payload map[string]any
	if err = json.Unmarshal(buffer.Bytes(), &payload); err != nil {
		t.Fatalf("解析 CLI JSON 输出失败: %v, output=%s", err, output)
	}
	return payload, nil, output
}

func newCLITestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18032,
		ProjectName:    "nexus-cli-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateCLISQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开 CLI 测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, cliMigrationDir(t)); err != nil {
		t.Fatalf("执行 CLI migration 失败: %v", err)
	}
}

func cliMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位 CLI 测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}

func asMap(t *testing.T, value any) map[string]any {
	t.Helper()

	item, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("输出结构不是对象: %#v", value)
	}
	return item
}

func asBool(t *testing.T, value any) bool {
	t.Helper()

	item, ok := value.(bool)
	if !ok {
		t.Fatalf("输出结构不是布尔值: %#v", value)
	}
	return item
}

func asString(t *testing.T, value any) string {
	t.Helper()

	item, ok := value.(string)
	if !ok {
		t.Fatalf("输出结构不是字符串: %#v", value)
	}
	return item
}

func assertAgentNames(t *testing.T, value any, expected ...string) {
	t.Helper()

	names := collectAgentNames(t, value)
	for _, item := range expected {
		if _, ok := names[item]; !ok {
			t.Fatalf("Agent 列表缺少 %q: %+v", item, names)
		}
	}
}

func assertAgentNamesAbsent(t *testing.T, value any, unexpected ...string) {
	t.Helper()

	names := collectAgentNames(t, value)
	for _, item := range unexpected {
		if _, ok := names[item]; ok {
			t.Fatalf("Agent 列表不应包含 %q: %+v", item, names)
		}
	}
}

func collectAgentNames(t *testing.T, value any) map[string]struct{} {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("agent 列表结构不正确: %#v", value)
	}
	result := make(map[string]struct{}, len(items))
	for _, raw := range items {
		item := asMap(t, raw)
		result[asString(t, item["name"])] = struct{}{}
	}
	return result
}
