package config

import (
	"os"
	"path/filepath"
	"testing"
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

func TestParseEnvBytes_Basic(t *testing.T) {
	raw := []byte("FOO=bar\nBAZ=123\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO=%q, want bar", m["FOO"])
	}
	if m["BAZ"] != "123" {
		t.Errorf("BAZ=%q, want 123", m["BAZ"])
	}
}

func TestParseEnvBytes_Comments(t *testing.T) {
	raw := []byte("# 这是注释\nFOO=bar\n# 另一条注释\nBAZ=qux\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Errorf("got %d entries, want 2", len(m))
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO=%q, want bar", m["FOO"])
	}
}

func TestParseEnvBytes_InlineComments(t *testing.T) {
	raw := []byte("FOO=bar # 这是一个注释\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO=%q, want bar (inline comment stripped)", m["FOO"])
	}
}

func TestParseEnvBytes_SingleQuoted(t *testing.T) {
	raw := []byte(`FOO='hello world'` + "\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "hello world" {
		t.Errorf("FOO=%q, want 'hello world'", m["FOO"])
	}
}

func TestParseEnvBytes_DoubleQuoted(t *testing.T) {
	raw := []byte(`FOO="hello world"` + "\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "hello world" {
		t.Errorf("FOO=%q, want 'hello world'", m["FOO"])
	}
}

func TestParseEnvBytes_DoubleQuotedEscapes(t *testing.T) {
	raw := []byte(`FOO="line1\nline2"` + "\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "line1\nline2" {
		t.Errorf("FOO=%q, want 'line1\\nline2'", m["FOO"])
	}
}

func TestParseEnvBytes_ExportPrefix(t *testing.T) {
	raw := []byte("export FOO=bar\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO=%q, want bar", m["FOO"])
	}
}

func TestParseEnvBytes_BlankLines(t *testing.T) {
	raw := []byte("\n\nFOO=bar\n\nBAZ=qux\n\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Errorf("got %d entries, want 2", len(m))
	}
}

func TestParseEnvBytes_VarExpansion(t *testing.T) {
	raw := []byte("BASE=/opt\nPATH=${BASE}/bin\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["PATH"] != "/opt/bin" {
		t.Errorf("PATH=%q, want /opt/bin", m["PATH"])
	}
}

func TestParseEnvBytes_SimpleVarExpansion(t *testing.T) {
	os.Setenv("NEXUS_TEST_EXT", "external")
	defer os.Unsetenv("NEXUS_TEST_EXT")

	raw := []byte(`URL="https://$NEXUS_TEST_EXT/api"` + "\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["URL"] != "https://external/api" {
		t.Errorf("URL=%q, want https://external/api", m["URL"])
	}
}

func TestParseEnvBytes_WindowsLineEndings(t *testing.T) {
	raw := []byte("FOO=bar\r\nBAZ=qux\r\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO=%q, want bar", m["FOO"])
	}
	if m["BAZ"] != "qux" {
		t.Errorf("BAZ=%q, want qux", m["BAZ"])
	}
}

func TestParseEnvBytes_EscapedDollar(t *testing.T) {
	raw := []byte(`FOO=\${BAR}` + "\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "${BAR}" {
		t.Errorf("FOO=%q, want '${BAR}'", m["FOO"])
	}
}

func TestParseEnvBytes_YamlColon(t *testing.T) {
	raw := []byte("FOO: bar\n")
	m, err := parseEnvBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO=%q, want bar", m["FOO"])
	}
}

func TestParseEnvBytes_UnterminatedQuote(t *testing.T) {
	raw := []byte(`FOO="unterminated` + "\n")
	_, err := parseEnvBytes(raw)
	if err == nil {
		t.Error("expected error for unterminated quote")
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
