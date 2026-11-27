// cli_test.go 覆盖 CLI 应用入口、配置环境、配置代理与输出渲染测试。
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	appruntime "github.com/nexus-research-lab/nexus/internal/app/runtime"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	_ "modernc.org/sqlite"
)

func TestHostManagedCLIScopeRejectsSelectionOverrides(t *testing.T) {
	t.Setenv(nexusctlUserIDEnvName, "user-owner")
	t.Setenv(nexusRuntimeScopeModeEnvName, runtimeScopeModeUserScoped)

	testCases := [][]string{
		{"--scope-user-id", "user-owner", "agent", "list"},
		{"--global-scope", "--scope-user-id", "", "agent", "list"},
	}
	for _, args := range testCases {
		command, err := New(config.Config{})
		if err != nil {
			t.Fatalf("创建 CLI 命令失败: %v", err)
		}
		command.SetArgs(args)
		err = command.Execute()
		if err == nil || !strings.Contains(err.Error(), hostManagedScopeOverrideError) {
			t.Fatalf("宿主注入作用域应拒绝显式覆盖: args=%q err=%v", args, err)
		}
	}
}

func TestCLIExecuteReleasesDatabasesAfterSuccessAndFailure(t *testing.T) {
	testCases := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{name: "success", args: []string{"agent", "list"}},
		{name: "command error", args: []string{"agent", "get", "missing-agent"}, wantError: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := newCLITestConfig(t)
			migrateCLISQLite(t, cfg.DatabaseURL)
			app, err := New(cfg)
			if err != nil {
				t.Fatalf("创建 CLI 应用失败: %v", err)
			}
			app.SetArgs(testCase.args)
			_, _, executeErr := captureCLIStreams(t, app)
			if (executeErr != nil) != testCase.wantError {
				t.Fatalf("CLI 执行结果错误: err=%v wantError=%t", executeErr, testCase.wantError)
			}
			if err := os.Remove(cfg.DatabaseURL); err != nil {
				t.Fatalf("CLI 执行后数据库仍被占用: %v", err)
			}
		})
	}
}

func TestLoadConfigFallsBackToCanonicalUsersRoot(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	runtimeRoot := appfs.UserRuntimeRootAt(stateRoot, "user_demo")
	t.Setenv(appfs.NexusStateRootEnvName, "")
	t.Setenv(nexusConfigDirEnvName, runtimeRoot)
	t.Setenv(nexusctlWorkspacePathEnvName, filepath.Join(stateRoot, "shared-workspaces", "project-a"))
	t.Setenv(workspacePathEnvName, filepath.Join(stateRoot, "shared-workspaces", "project-a"))
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NEXUS_APP_MODE", "")

	cfg := LoadConfig()

	if got, want := cfg.WorkspacePath, filepath.Join(stateRoot, "users"); got != want {
		t.Fatalf("nexusctl workspace base = %q, want %q", got, want)
	}
}

func TestLoadConfigurationConfigDoesNotRestoreHostRootsForRuntimeBroker(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	runtimeRoot := appfs.UserRuntimeRootAt(stateRoot, "user_demo")
	t.Setenv(appfs.NexusStateRootEnvName, "")
	t.Setenv(nexusConfigDirEnvName, runtimeRoot)
	t.Setenv(workspacePathEnvName, filepath.Join(stateRoot, "users", "user_demo", "workspace", "agent-a"))
	t.Setenv(protocol.NexusConfigBrokerURLEnvName, "http://127.0.0.1:8010/nexus/v1/internal/runtime/configuration")
	t.Setenv(protocol.NexusConfigCapabilityTokenEnvName, "runtime-token")

	_ = LoadConfigurationConfig()

	if got := os.Getenv(appfs.NexusStateRootEnvName); got != "" {
		t.Fatalf("broker 模式不应恢复宿主状态根: %q", got)
	}
}

func TestRuntimeConfigurationRequestsReachHostValidation(t *testing.T) {
	manager := runtimectx.NewManager()
	const session, round = "broker-test-session", "broker-test-round"
	if err := manager.StartRound(t.Context(), session, round, nil); err != nil {
		t.Fatal(err)
	}
	defer manager.MarkRoundFinished(session, round)
	service := configurationsvc.NewService(config.Config{}, nil, nil, nil, nil, nil, nil, nil, manager)
	token, err := service.IssueRuntimeCapability(configurationsvc.Actor{
		OwnerUserID: "owner", AgentID: "main", LeaseSessionKey: session, LeaseRoundID: round, RoundLeaseRequired: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(appruntime.NewConfigurationHandler(service, nil))
	defer server.Close()
	controller := &runtimeConfigurationController{endpoint: server.URL, token: token, client: server.Client()}
	request := configurationsvc.ChangeRequest{Domain: "agents", Operation: "update_self_profile"}
	calls := map[string]func(context.Context) error{
		"inspect all": func(ctx context.Context) error { _, err := controller.Inspect(ctx, nil, false); return err },
		"inspect members": func(ctx context.Context) error {
			_, err := controller.Inspect(ctx, []string{"members"}, false)
			return err
		},
		"inspect agents": func(ctx context.Context) error {
			_, err := controller.Inspect(ctx, []string{"agents"}, false)
			return err
		},
		"history": func(ctx context.Context) error { _, err := controller.History(ctx, "members", 10); return err },
		"plan":    func(ctx context.Context) error { _, err := controller.Plan(ctx, request); return err },
		"apply": func(ctx context.Context) error {
			_, err := controller.Apply(ctx, request, configurationsvc.CLIApplyOptions{})
			return err
		},
		"review": func(ctx context.Context) error { _, err := controller.Review(ctx, "cfg-broker-test"); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			// 不装配 Agent 服务，让请求在通过真实解码后安全停在身份校验。
			if err := call(t.Context()); err == nil || !strings.Contains(err.Error(), configurationsvc.ErrMainAgentRequired.Error()) {
				t.Fatalf("应进入宿主业务校验，实际错误: %v", err)
			}
		})
	}
	if _, err := controller.Reconcile(t.Context(), configurationsvc.ReconcileRequest{}); err == nil || !strings.Contains(err.Error(), "不能提交人工配置 reconcile") {
		t.Fatalf("runtime 必须拒绝人工核对命令: %v", err)
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
