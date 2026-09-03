// =====================================================
// @File   ：output_test.go
// @Date   ：2026-04-23 10:17
// @Author ：leemysw
// 2026-04-23 10:17   Create
// =====================================================

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"

	_ "modernc.org/sqlite"
)

func TestCLIJSONFlagOutputsCompactJSON(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	command, err := New(cfg)
	if err != nil {
		t.Fatalf("创建 CLI 命令失败: %v", err)
	}
	command.SetArgs([]string{"--json", "agent", "list"})

	stdout, stderr, executeErr := captureCLIStreams(t, command)
	if executeErr != nil {
		t.Fatalf("执行 --json agent list 失败: %v, stderr=%s", executeErr, stderr)
	}
	if strings.Contains(stdout, "\n  ") {
		t.Fatalf("--json 输出不应包含缩进: %s", stdout)
	}

	var payload map[string]any
	if err = json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("解析 JSON 失败: %v, stdout=%s", err, stdout)
	}
	if payload["success"] != true {
		t.Fatalf("JSON 输出应带 success=true: %+v", payload)
	}
}

func TestCLIUsageErrorUsesExitCode64AndStderrJSON(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	command, err := New(cfg)
	if err != nil {
		t.Fatalf("创建 CLI 命令失败: %v", err)
	}
	command.SetArgs([]string{"--json", "room", "get"})

	stdout, _, executeErr := captureCLIStreams(t, command)
	if stdout != "" {
		t.Fatalf("usage 错误时 stdout 应为空: %s", stdout)
	}
	if executeErr == nil {
		t.Fatal("缺少参数时应返回 usage 错误")
	}
	if ExitCode(executeErr) != exitCodeUsage {
		t.Fatalf("usage 错误应返回 64，实际=%d err=%v", ExitCode(executeErr), executeErr)
	}

	var stderr bytes.Buffer
	WriteCommandError(&stderr, executeErr, true)

	var payload map[string]any
	if err = json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("解析 stderr JSON 失败: %v, stderr=%s", err, stderr.String())
	}
	if payload["success"] != false {
		t.Fatalf("stderr JSON 应标记 success=false: %+v", payload)
	}
	errorItem, ok := payload["error"].(map[string]any)
	if !ok || errorItem["kind"] != cliErrorKindUsage {
		t.Fatalf("stderr JSON 应标记 usage 错误: %+v", payload)
	}
}

func TestCLIExecutionErrorUsesExitCode1AndStderrJSON(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	command, err := New(cfg)
	if err != nil {
		t.Fatalf("创建 CLI 命令失败: %v", err)
	}
	command.SetArgs([]string{"--json", "agent", "get", "missing-agent"})

	stdout, _, executeErr := captureCLIStreams(t, command)
	if stdout != "" {
		t.Fatalf("执行错误时 stdout 应为空: %s", stdout)
	}
	if executeErr == nil {
		t.Fatal("读取不存在的 agent 应返回执行错误")
	}
	if ExitCode(executeErr) != exitCodeExecution {
		t.Fatalf("执行错误应返回 1，实际=%d err=%v", ExitCode(executeErr), executeErr)
	}

	var stderr bytes.Buffer
	WriteCommandError(&stderr, executeErr, true)

	var payload map[string]any
	if err = json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("解析 stderr JSON 失败: %v, stderr=%s", err, stderr.String())
	}
	errorItem, ok := payload["error"].(map[string]any)
	if !ok || errorItem["kind"] != cliErrorKindExecution {
		t.Fatalf("stderr JSON 应标记 execution 错误: %+v", payload)
	}
}

func captureCLIStreams(t *testing.T, command interface{ Execute() error }) (string, string, error) {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建 stdout 管道失败: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建 stderr 管道失败: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	stdoutCapture := startCLIPipeCapture(stdoutReader)
	stderrCapture := startCLIPipeCapture(stderrReader)
	executeErr := command.Execute()
	stdout := finishCLIPipeCapture(t, stdoutWriter, stdoutCapture)
	stderr := finishCLIPipeCapture(t, stderrWriter, stderrCapture)

	return strings.TrimSpace(string(stdout)), strings.TrimSpace(string(stderr)), executeErr
}

func newCLITestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("HOME", root)
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
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
	handlertest.MigrateSQLiteFromDir(t, databaseURL, cliMigrationDir(t))
}

func cliMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位 CLI 测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}

type cliPipeCapture struct {
	payload []byte
	err     error
}

func startCLIPipeCapture(reader *os.File) <-chan cliPipeCapture {
	result := make(chan cliPipeCapture, 1)
	go func() {
		var buffer bytes.Buffer
		_, readErr := buffer.ReadFrom(reader)
		if closeErr := reader.Close(); readErr == nil {
			readErr = closeErr
		}
		result <- cliPipeCapture{payload: buffer.Bytes(), err: readErr}
	}()
	return result
}

func finishCLIPipeCapture(
	t *testing.T,
	writer *os.File,
	result <-chan cliPipeCapture,
) []byte {
	t.Helper()
	if err := writer.Close(); err != nil {
		t.Fatalf("关闭 CLI 输出管道失败: %v", err)
	}
	captured := <-result
	if captured.err != nil {
		t.Fatalf("读取 CLI 输出失败: %v", captured.err)
	}
	return captured.payload
}
