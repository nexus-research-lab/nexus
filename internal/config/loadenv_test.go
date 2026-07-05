package config

import (
	"os"
	"path/filepath"
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
