// config_test.go 覆盖配置加载、环境变量、Relay、Remote 与 Computer Use 配置测试。
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
		{
			name: "empty value before comment and statement",
			raw:  "WORKSPACE_PATH=\n# Browser Login\nHOST=0.0.0.0\n",
			want: map[string]string{"WORKSPACE_PATH": "", "HOST": "0.0.0.0"},
		},
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

func TestLoadMultiplayerDisabled(t *testing.T) {
	for _, value := range []string{"true", "false", ""} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("NEXUS_MULTIPLAYER_DISABLED", value)
			if got := Load().MultiplayerDisabled; got != (value == "true") {
				t.Fatalf("MultiplayerDisabled = %v for %q", got, value)
			}
		})
	}
}

func TestLoadRelayConfigurationIsOptional(t *testing.T) {
	t.Setenv("NEXUS_RELAY_URL", "")
	t.Setenv("NEXUS_RELAY_REQUEST_TIMEOUT_SECONDS", "")

	cfg := Load()
	if cfg.RelayURL != "" {
		t.Fatalf("RelayURL = %q, want disabled", cfg.RelayURL)
	}
	if cfg.RelayRequestTimeoutSeconds != 5 {
		t.Fatalf("RelayRequestTimeoutSeconds = %d, want 5", cfg.RelayRequestTimeoutSeconds)
	}
}

func TestLoadRelayConfiguration(t *testing.T) {
	t.Setenv("NEXUS_RELAY_URL", " https://relay.example.com ")
	t.Setenv("NEXUS_RELAY_REQUEST_TIMEOUT_SECONDS", "9")

	cfg := Load()
	if cfg.RelayURL != "https://relay.example.com" {
		t.Fatalf("RelayURL = %q", cfg.RelayURL)
	}
	if cfg.RelayRequestTimeoutSeconds != 9 {
		t.Fatalf("RelayRequestTimeoutSeconds = %d, want 9", cfg.RelayRequestTimeoutSeconds)
	}
}

func TestDesktopSandboxIsBuiltIn(t *testing.T) {
	t.Setenv("NEXUS_APP_MODE", "desktop")
	t.Setenv("NEXUS_DESKTOP_SANDBOX_ENABLED", "false")
	if !Load().DesktopSandboxEnabled {
		t.Fatal("desktop sandbox must remain enabled without an environment opt-in")
	}
}

func TestComputerUseIsOptIn(t *testing.T) {
	t.Setenv("NEXUS_COMPUTER_USE_ENABLED", "")
	t.Setenv("NEXUS_COMPUTER_USE_BINARY", "")
	cfg := Load()
	if cfg.ComputerUseEnabled {
		t.Fatal("Computer Use must be disabled when the flag is empty")
	}
	if cfg.ComputerUseBinaryPath != "" {
		t.Fatalf("ComputerUseBinaryPath = %q, want empty", cfg.ComputerUseBinaryPath)
	}
}

func TestComputerUseConfiguration(t *testing.T) {
	t.Setenv("NEXUS_COMPUTER_USE_ENABLED", "true")
	t.Setenv("NEXUS_COMPUTER_USE_BINARY", " /Applications/Nexus CUA.app/Contents/MacOS/nexuscua ")
	cfg := Load()
	if !cfg.ComputerUseEnabled {
		t.Fatal("ComputerUseEnabled = false, want true")
	}
	if cfg.ComputerUseBinaryPath != "/Applications/Nexus CUA.app/Contents/MacOS/nexuscua" {
		t.Fatalf("ComputerUseBinaryPath = %q", cfg.ComputerUseBinaryPath)
	}
}
