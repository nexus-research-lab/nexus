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
	"strings"
	"testing"
)

func TestCLIJSONFlagOutputsCompactJSON(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	command, err := New(cfg)
	if err != nil {
		t.Fatalf("创建 CLI 命令失败: %v", err)
	}
	command.SetArgs([]string{"--json", "auth", "status"})

	stdout, stderr, executeErr := captureCLIStreams(t, command)
	if executeErr != nil {
		t.Fatalf("执行 --json auth status 失败: %v, stderr=%s", executeErr, stderr)
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

	executeErr := command.Execute()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	_, _ = stdoutBuffer.ReadFrom(stdoutReader)
	_, _ = stderrBuffer.ReadFrom(stderrReader)
	_ = stdoutReader.Close()
	_ = stderrReader.Close()

	return strings.TrimSpace(stdoutBuffer.String()), strings.TrimSpace(stderrBuffer.String()), executeErr
}
