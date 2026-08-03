package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func writeTestEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseEnvBytes(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		env    map[string]string
		want   map[string]string
		hasErr bool
	}{
		{name: "basic", raw: "FOO=bar\nBAZ=123\n", want: map[string]string{"FOO": "bar", "BAZ": "123"}},
		{name: "comments", raw: "# 这是注释\nFOO=bar\n# 另一条注释\nBAZ=qux\n", want: map[string]string{"FOO": "bar", "BAZ": "qux"}},
		{name: "inline comments", raw: "FOO=bar # 这是一个注释\n", want: map[string]string{"FOO": "bar"}},
		{name: "single quoted", raw: "FOO='hello world'\n", want: map[string]string{"FOO": "hello world"}},
		{name: "double quoted", raw: "FOO=\"hello world\"\n", want: map[string]string{"FOO": "hello world"}},
		{name: "double quoted escapes", raw: `FOO="line1\nline2"` + "\n", want: map[string]string{"FOO": "line1\nline2"}},
		{name: "export prefix", raw: "export FOO=bar\n", want: map[string]string{"FOO": "bar"}},
		{name: "blank lines", raw: "\n\nFOO=bar\n\nBAZ=qux\n\n", want: map[string]string{"FOO": "bar", "BAZ": "qux"}},
		{name: "var expansion", raw: "BASE=/opt\nPATH=${BASE}/bin\n", want: map[string]string{"BASE": "/opt", "PATH": "/opt/bin"}},
		{
			name: "simple var expansion",
			raw:  "URL=\"https://$NEXUS_TEST_EXT/api\"\n",
			env:  map[string]string{"NEXUS_TEST_EXT": "external"},
			want: map[string]string{"URL": "https://external/api"},
		},
		{name: "windows line endings", raw: "FOO=bar\r\nBAZ=qux\r\n", want: map[string]string{"FOO": "bar", "BAZ": "qux"}},
		{name: "escaped dollar", raw: `FOO=\${BAR}` + "\n", want: map[string]string{"FOO": "${BAR}"}},
		{name: "yaml colon", raw: "FOO: bar\n", want: map[string]string{"FOO": "bar"}},
		{name: "unterminated quote", raw: `FOO="unterminated` + "\n", hasErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			got, err := parseEnvBytes([]byte(test.raw))
			if test.hasErr {
				if err == nil {
					t.Fatal("parseEnvBytes() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseEnvBytes() error = %v", err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("parseEnvBytes() = %#v, want %#v", got, test.want)
			}
			for key, want := range test.want {
				if got[key] != want {
					t.Fatalf("parseEnvBytes()[%q] = %q, want %q", key, got[key], want)
				}
			}
		})
	}
}

func TestLoadDotEnv_FromFile(t *testing.T) {
	path := writeTestEnv(t, "NEXUS_LOAD_TEST_HELLO=world\n")
	os.Unsetenv("NEXUS_LOAD_TEST_HELLO")

	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if v := os.Getenv("NEXUS_LOAD_TEST_HELLO"); v != "world" {
		t.Errorf("got %q, want world", v)
	}
}

func TestLoadDotEnv_DoesNotOverride(t *testing.T) {
	os.Setenv("NEXUS_NO_OVERRIDE", "original")
	defer os.Unsetenv("NEXUS_NO_OVERRIDE")

	path := writeTestEnv(t, "NEXUS_NO_OVERRIDE=from_env_file\n")
	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if v := os.Getenv("NEXUS_NO_OVERRIDE"); v != "original" {
		t.Errorf("got %q, want 'original' (should not override)", v)
	}
}

func TestLoadDotEnv_MissingFile(t *testing.T) {
	// 不存在的文件应该静默跳过，不报错
	if err := LoadDotEnv("/nonexistent/.env"); err != nil {
		t.Errorf("expected nil error for missing file, got %v", err)
	}
}

func TestLoadMessageDebugStreamEvent(t *testing.T) {
	t.Setenv("MESSAGE_DEBUG_STREAM_EVENT", "true")

	cfg := Load()

	if !cfg.MessageDebugStreamEvent {
		t.Fatalf("MESSAGE_DEBUG_STREAM_EVENT=true 应开启 StreamEvent 日志")
	}
}

func TestLoadRuntimeIdleSessionSettings(t *testing.T) {
	t.Setenv("RUNTIME_ROUND_IDLE_TIMEOUT_SECONDS", "120")
	t.Setenv("RUNTIME_IDLE_SESSION_TTL_SECONDS", "60")
	t.Setenv("RUNTIME_IDLE_SESSION_SWEEP_SECONDS", "15")

	cfg := Load()

	if cfg.RuntimeRoundIdleTimeout() != 120*time.Second {
		t.Fatalf("RuntimeRoundIdleTimeout = %s, want 120s", cfg.RuntimeRoundIdleTimeout())
	}
	if cfg.RuntimeIdleSessionTTL() != 60*time.Second {
		t.Fatalf("RuntimeIdleSessionTTL = %s, want 60s", cfg.RuntimeIdleSessionTTL())
	}
	if cfg.RuntimeIdleSessionSweepInterval() != 15*time.Second {
		t.Fatalf("RuntimeIdleSessionSweepInterval = %s, want 15s", cfg.RuntimeIdleSessionSweepInterval())
	}
}

func TestLoadWorkspacePathUsesRuntimeSettingsWhenEnvEmpty(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "custom-workspace")
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	t.Setenv("WORKSPACE_PATH", "")
	if _, err := SaveRuntimeSettings(RuntimeSettings{
		WorkspacePath:    workspacePath,
		AppliedUsersPath: workspacePath,
	}); err != nil {
		t.Fatalf("写入 runtime settings 失败: %v", err)
	}

	cfg := Load()

	if cfg.WorkspacePath != workspacePath {
		t.Fatalf("WorkspacePath = %q, want %q", cfg.WorkspacePath, workspacePath)
	}
}

func TestSaveRuntimeSettingsUsesPrivatePermissions(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	appliedPath := filepath.Join(stateRoot, "users")

	if _, err := SaveRuntimeSettings(RuntimeSettings{
		WorkspacePath:      "/tmp/workspace",
		AppliedUsersPath:   appliedPath,
		PendingUsersPath:   "/tmp/pending-users",
		MigratingUsersPath: "/tmp/migrating-users",
	}); err != nil {
		t.Fatalf("写入 runtime settings 失败: %v", err)
	}
	settings, err := LoadRuntimeSettings()
	if err != nil {
		t.Fatalf("读回 runtime settings 失败: %v", err)
	}
	if settings.AppliedUsersPath != appliedPath {
		t.Fatalf("AppliedUsersPath = %q, want %q", settings.AppliedUsersPath, appliedPath)
	}
	if settings.PendingUsersPath != "/tmp/pending-users" {
		t.Fatalf("PendingUsersPath = %q", settings.PendingUsersPath)
	}
	if settings.MigratingUsersPath != "/tmp/migrating-users" {
		t.Fatalf("MigratingUsersPath = %q", settings.MigratingUsersPath)
	}
	configInfo, err := os.Stat(filepath.Join(stateRoot, "app", "config"))
	if err != nil {
		t.Fatalf("读取 runtime settings 配置目录失败: %v", err)
	}
	if runtime.GOOS != "windows" && configInfo.Mode().Perm() != 0o700 {
		t.Fatalf("runtime settings 配置目录权限错误: %o", configInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(RuntimeSettingsPath())
	if err != nil {
		t.Fatalf("读取 runtime settings 文件失败: %v", err)
	}
	if runtime.GOOS != "windows" && fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("runtime settings 文件权限错误: %o", fileInfo.Mode().Perm())
	}
}

func TestSaveRuntimeSettingsRejectsSymlinkedConfigDirectory(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	outsideRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	if err := os.MkdirAll(filepath.Join(stateRoot, "app"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(stateRoot, "app", "config")); err != nil {
		t.Skipf("当前平台不支持符号链接: %v", err)
	}

	if _, err := SaveRuntimeSettings(RuntimeSettings{WorkspacePath: "/tmp/workspace"}); err == nil {
		t.Fatal("runtime settings 不应写入符号链接配置目录")
	}
	if _, err := os.Stat(filepath.Join(outsideRoot, runtimeSettingsFileName)); !os.IsNotExist(err) {
		t.Fatalf("符号链接目标不应收到 runtime settings: %v", err)
	}
}

func TestLoadWorkspacePathKeepsExplicitEnv(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".nexus")
	persistedPath := filepath.Join(root, "persisted-workspace")
	envPath := filepath.Join(root, "env-workspace")
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", configDir)
	t.Setenv("WORKSPACE_PATH", envPath)
	if _, err := SaveRuntimeSettings(RuntimeSettings{
		WorkspacePath:    persistedPath,
		AppliedUsersPath: persistedPath,
	}); err != nil {
		t.Fatalf("写入 runtime settings 失败: %v", err)
	}

	cfg := Load()

	if cfg.WorkspacePath != envPath {
		t.Fatalf("WorkspacePath = %q, want explicit env %q", cfg.WorkspacePath, envPath)
	}
}

func TestLoadWorkspacePathOverridesDesktopDefaultEnv(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".nexus")
	persistedPath := filepath.Join(root, "persisted-workspace")
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", configDir)
	t.Setenv("NEXUS_APP_MODE", "desktop")
	t.Setenv("WORKSPACE_PATH", filepath.Join(configDir, "workspace"))
	if _, err := SaveRuntimeSettings(RuntimeSettings{
		WorkspacePath:    persistedPath,
		AppliedUsersPath: persistedPath,
	}); err != nil {
		t.Fatalf("写入 runtime settings 失败: %v", err)
	}

	cfg := Load()

	if cfg.WorkspacePath != persistedPath {
		t.Fatalf("WorkspacePath = %q, want persisted %q", cfg.WorkspacePath, persistedPath)
	}
}

func TestLoadWorkspacePathIgnoresPendingUsersRoot(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".nexus")
	activePath := filepath.Join(root, "active-users")
	pendingPath := filepath.Join(root, "pending-users")
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", configDir)
	t.Setenv("NEXUS_APP_MODE", "desktop")
	t.Setenv("WORKSPACE_PATH", filepath.Join(configDir, "users"))
	if _, err := SaveRuntimeSettings(RuntimeSettings{
		WorkspacePath:    activePath,
		AppliedUsersPath: activePath,
		PendingUsersPath: pendingPath,
	}); err != nil {
		t.Fatalf("写入 runtime settings 失败: %v", err)
	}

	cfg := Load()

	if cfg.WorkspacePath != activePath {
		t.Fatalf("待迁移 users 根提前生效: got=%q want=%q", cfg.WorkspacePath, activePath)
	}
}

func TestLoadWorkspacePathIgnoresMigratingUsersRoot(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".nexus")
	activePath := filepath.Join(root, "active-users")
	migratingPath := filepath.Join(root, "migrating-users")
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", configDir)
	t.Setenv("NEXUS_APP_MODE", "desktop")
	t.Setenv("WORKSPACE_PATH", filepath.Join(configDir, "users"))
	if _, err := SaveRuntimeSettings(RuntimeSettings{
		WorkspacePath:      activePath,
		AppliedUsersPath:   activePath,
		MigratingUsersPath: migratingPath,
	}); err != nil {
		t.Fatalf("写入 runtime settings 失败: %v", err)
	}

	cfg := Load()

	if cfg.WorkspacePath != activePath {
		t.Fatalf("迁移中的 users 根提前生效: got=%q want=%q", cfg.WorkspacePath, activePath)
	}
}

func TestLoadMapsLegacyHostPathsIntoAppDirectory(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	t.Setenv("CACHE_FILE_DIR", filepath.Join(stateRoot, "cache"))
	t.Setenv("LOG_PATH", filepath.Join(stateRoot, "logs", "legacy.log"))
	t.Setenv("DATABASE_URL", "sqlite:///"+filepath.Join(stateRoot, "data", "nexus.db"))

	cfg := Load()

	if cfg.CacheFileDir != filepath.Join(stateRoot, "app", "cache") {
		t.Fatalf("CacheFileDir = %q, want app cache", cfg.CacheFileDir)
	}
	if cfg.LogPath != filepath.Join(stateRoot, "app", "logs", "legacy.log") {
		t.Fatalf("LogPath = %q, want app logs", cfg.LogPath)
	}
	if cfg.DatabaseURL != "sqlite:///"+filepath.Join(stateRoot, "app", "data", "nexus.db") {
		t.Fatalf("DatabaseURL = %q, want migrated sqlite path", cfg.DatabaseURL)
	}
}

func TestLoadDoesNotTreatAgentRuntimeWorkspaceAsHostRoot(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	agentWorkspace := filepath.Join(stateRoot, "users", "__system__", "workspace", "nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	t.Setenv("WORKSPACE_PATH", agentWorkspace)
	t.Setenv("NEXUSCTL_WORKSPACE_PATH", agentWorkspace)

	cfg := Load()

	want := filepath.Join(stateRoot, "users")
	if cfg.WorkspacePath != want {
		t.Fatalf("Agent runtime workspace 被误作宿主根: got=%q want=%q", cfg.WorkspacePath, want)
	}
}

func TestLoadDotEnv_Complex(t *testing.T) {
	content := `# 应用配置
export APP_NAME=nexus

# 数据库
DB_DRIVER=postgres
DB_URL="postgres://localhost:5432/$APP_NAME"

# 带注释的行
PORT=8010 # HTTP 端口

# 带引号的密码
SECRET='p@ss=w0rd#123'

# 带转义的字符串
MULTILINE="line1\nline2"
`
	path := writeTestEnv(t, content)
	os.Unsetenv("APP_NAME")
	os.Unsetenv("DB_DRIVER")
	os.Unsetenv("DB_URL")
	os.Unsetenv("PORT")
	os.Unsetenv("SECRET")
	os.Unsetenv("MULTILINE")

	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct{ key, want string }{
		{"APP_NAME", "nexus"},
		{"DB_DRIVER", "postgres"},
		{"DB_URL", "postgres://localhost:5432/nexus"},
		{"PORT", "8010"},
		{"SECRET", "p@ss=w0rd#123"},
		{"MULTILINE", "line1\nline2"},
	}
	for _, tc := range tests {
		if v := os.Getenv(tc.key); v != tc.want {
			t.Errorf("%s=%q, want %q", tc.key, v, tc.want)
		}
	}
}
