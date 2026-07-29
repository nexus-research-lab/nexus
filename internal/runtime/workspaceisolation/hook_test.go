package workspaceisolation

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestWorkspacePolicyHookAllowsOwnWorkspaceAndDeniesOtherUser(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "users", "owner-a", "workspace", "agent-a")
	otherWorkspace := filepath.Join(root, "users", "owner-b", "workspace", "agent-b")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	policy := testPolicy(t, workspace)
	callback := workspacePolicyCallback(ModeEnforce, policy)

	allowed, err := callback(context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPreToolUse,
		CWD:       workspace,
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": filepath.Join(workspace, "README.md")},
	}, "tool-allow")
	if err != nil {
		t.Fatal(err)
	}
	if allowed.Continue != nil || allowed.SpecificOutput != nil {
		t.Fatalf("own workspace decision = %#v", allowed)
	}

	denied, err := callback(context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPreToolUse,
		CWD:       workspace,
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": filepath.Join(otherWorkspace, "secret.txt")},
	}, "tool-deny")
	if err != nil {
		t.Fatal(err)
	}
	if denied.SpecificOutput == nil ||
		denied.SpecificOutput.PermissionDecision != sdkpermission.BehaviorDeny {
		t.Fatalf("other workspace decision = %#v", denied)
	}
}

func TestWorkspacePolicyHookOnlyAllowsCanonicalSessionSummaryEdit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	workspace := filepath.Join(root, "users", "owner-a", "workspace", "agent-a")
	summaryPath := filepath.Join(
		root,
		"users",
		"owner-a",
		"runtime",
		"projects",
		"project-a",
		"session-a",
		"session-memory",
		"summary.md",
	)
	if err := os.MkdirAll(filepath.Dir(summaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(summaryPath, []byte("summary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(root, "outside.md")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkSummaryPath := filepath.Join(
		root,
		"users",
		"owner-a",
		"runtime",
		"projects",
		"project-a",
		"session-link",
		"session-memory",
		"summary.md",
	)
	if err := os.MkdirAll(filepath.Dir(symlinkSummaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePath, symlinkSummaryPath); err != nil {
		t.Fatal(err)
	}
	policy := testPolicy(t, workspace)
	policy.Identity.TempDir = filepath.Join(
		root,
		"users",
		"owner-a",
		"runtime",
		"tmp",
	)
	for _, test := range []struct {
		name     string
		toolName string
		path     string
		denied   bool
	}{
		{name: "exact Edit", toolName: "Edit", path: summaryPath},
		{name: "Write remains denied", toolName: "Write", path: summaryPath, denied: true},
		{
			name:     "adjacent runtime file remains denied",
			toolName: "Edit",
			path:     filepath.Join(filepath.Dir(summaryPath), "state.json"),
			denied:   true,
		},
		{
			name:     "other owner remains denied",
			toolName: "Edit",
			path: filepath.Join(
				root,
				"users",
				"owner-b",
				"runtime",
				"projects",
				"project-b",
				"session-b",
				"session-memory",
				"summary.md",
			),
			denied: true,
		},
		{
			name:     "symlink escape remains denied",
			toolName: "Edit",
			path:     symlinkSummaryPath,
			denied:   true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			violation := inspectToolAccess(policy, sdkhook.Input{
				CWD:      workspace,
				ToolName: test.toolName,
				ToolInput: map[string]any{
					"file_path": test.path,
				},
			})
			if (violation != nil) != test.denied {
				t.Fatalf(
					"%s %q violation = %#v, denied=%v",
					test.toolName,
					test.path,
					violation,
					test.denied,
				)
			}
		})
	}
	claudePolicy := policy
	claudePolicy.RuntimeKind = "claude"
	if violation := inspectToolAccess(claudePolicy, sdkhook.Input{
		CWD:      workspace,
		ToolName: "Edit",
		ToolInput: map[string]any{
			"file_path": summaryPath,
		},
	}); violation == nil {
		t.Fatal("Claude runtime 不应获得 nxs session-memory 写权限")
	}
}

func TestWorkspacePolicyHookResolvesPendingPathThroughSymlink(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	otherWorkspace := filepath.Join(root, "other")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(otherWorkspace, filepath.Join(workspace, "escape")); err != nil {
		t.Fatal(err)
	}
	policy := testPolicy(t, workspace)
	violation := inspectToolAccess(policy, sdkhook.Input{
		CWD:      workspace,
		ToolName: "Write",
		ToolInput: map[string]any{
			"file_path": filepath.Join(workspace, "escape", "pending", "secret.txt"),
		},
	})
	if violation == nil {
		t.Fatal("pending path through symlink should be denied")
	}
}

func TestWorkspacePolicyHookChecksBashAndNexusctlWithoutBlockingSystemTools(t *testing.T) {
	workspace := t.TempDir()
	policy := testPolicy(t, workspace)
	for _, test := range []struct {
		name    string
		command string
		denied  bool
	}{
		{name: "ordinary command", command: "/usr/bin/git status >/dev/null", denied: false},
		{name: "command substitution syntax", command: "ps -u $(whoami) -o pid,ppid,cmd", denied: false},
		{name: "grep end anchor", command: "ls /proc | grep -E '^[0-9]+$' | wc -l", denied: false},
		{name: "awk field selector", command: "printf 'a b' | awk '{print $1}'", denied: false},
		{
			name: "dynamic proc path",
			command: "ls /proc | grep -E '^[0-9]+$' | while read pid; do " +
				"cmd=$(cat /proc/$pid/cmdline 2>/dev/null | tr '\\0' ' '); " +
				"if [ -n \"$cmd\" ]; then echo \"$pid $cmd\"; fi; done | head -50",
			denied: false,
		},
		{name: "dynamic workspace path", command: "cat ./logs/$name", denied: false},
		{
			name:    "dynamic outside prefix",
			command: "cat " + filepath.Join(filepath.Dir(workspace), "outside", "$name"),
			denied:  true,
		},
		{name: "relative escape", command: "cat ../../other/secret", denied: true},
		{name: "redirect escape", command: "printf secret > ../../other/secret", denied: true},
		{name: "home shorthand", command: "cat ~/secret", denied: true},
		{name: "named home shorthand", command: "cat ~other/secret", denied: true},
		{name: "shell variable", command: "cat $HOME/secret", denied: true},
		{name: "braced shell variable", command: "cat ${HOME}/secret", denied: true},
		{name: "cmd variable", command: `type %USERPROFILE%\secret`, denied: true},
		{name: "windows absolute path", command: `type C:\Users\other\secret`, denied: true},
		{name: "nexusctl broker pending", command: "nexusctl agent list", denied: true},
		{name: "global nexusctl", command: "nexusctl --scope global agents list", denied: true},
		{name: "forged owner", command: "NEXUSCTL_USER_ID=owner-b nexusctl agents list", denied: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			violation := inspectToolAccess(policy, sdkhook.Input{
				CWD:      workspace,
				ToolName: "Bash",
				ToolInput: map[string]any{
					"command": test.command,
				},
			})
			if (violation != nil) != test.denied {
				t.Fatalf("command %q violation = %#v, denied=%v", test.command, violation, test.denied)
			}
		})
	}
}

func TestWorkspacePolicyHookAllowsMainAgentNexusctl(t *testing.T) {
	workspace := t.TempDir()
	policy := testPolicy(t, workspace)
	policy.IsMainAgent = true

	for _, test := range []struct {
		name    string
		command string
		denied  bool
	}{
		{
			name:    "injected command path",
			command: `"$NEXUSCTL_COMMAND_PATH" --json agent list`,
		},
		{
			name:    "bare command path",
			command: "nexusctl --json room list",
		},
		{
			name:    "owner scoped user create",
			command: `"$NEXUSCTL_COMMAND_PATH" --json user create --username alice --password test-only`,
		},
		{
			name:    "forged owner",
			command: "NEXUSCTL_USER_ID=owner-b nexusctl --json agent list",
			denied:  true,
		},
		{
			name:    "global scope",
			command: "nexusctl --global-scope agent list",
			denied:  true,
		},
		{
			name:    "explicit owner scope",
			command: "nexusctl --scope-user-id owner-b agent list",
			denied:  true,
		},
		{
			name:    "forged workspace",
			command: "NEXUSCTL_WORKSPACE_PATH=/tmp/other nexusctl workspace list",
			denied:  true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			violation := inspectToolAccess(policy, sdkhook.Input{
				CWD:      workspace,
				ToolName: "Bash",
				ToolInput: map[string]any{
					"command": test.command,
				},
			})
			if (violation != nil) != test.denied {
				t.Fatalf("command %q violation = %#v, denied=%v", test.command, violation, test.denied)
			}
		})
	}
}

func TestWorkspacePolicyHookTerminatesForbiddenNexusctl(t *testing.T) {
	workspace := t.TempDir()
	callback := workspacePolicyCallback(ModeEnforce, testPolicy(t, workspace))

	output, err := callback(context.Background(), sdkhook.Input{
		CWD:      workspace,
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "nexusctl --json agent list",
		},
	}, "ordinary-tool")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil ||
		output.SpecificOutput.PermissionDecision != sdkpermission.BehaviorDeny {
		t.Fatalf("普通 Agent nexusctl 应被拒绝: %#v", output)
	}
	if output.Continue == nil || *output.Continue || output.StopReason == "" {
		t.Fatalf("控制面越界应终止当前 runtime turn: %#v", output)
	}
}

func TestWorkspacePolicyHookKeepsMainAgentScopeOverrideRecoverable(t *testing.T) {
	workspace := t.TempDir()
	policy := testPolicy(t, workspace)
	policy.IsMainAgent = true
	callback := workspacePolicyCallback(ModeEnforce, policy)

	output, err := callback(context.Background(), sdkhook.Input{
		CWD:      workspace,
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": `nexusctl --json --global-scope --scope-user-id "" user list`,
		},
	}, "main-agent-stale-scope")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil ||
		output.SpecificOutput.PermissionDecision != sdkpermission.BehaviorDeny {
		t.Fatalf("主智能体显式覆盖 owner scope 应拒绝本次调用: %#v", output)
	}
	if output.Continue != nil || output.StopReason != "" {
		t.Fatalf("主智能体旧作用域参数应允许同轮修正重试: %#v", output)
	}
	if output.SpecificOutput.PermissionDecisionReason != mainAgentNexusctlScopeDenial {
		t.Fatalf("主智能体应收到可执行的修正提示: %#v", output)
	}
}

func TestWorkspacePolicyHookAllowsSharedTemporaryRedirect(t *testing.T) {
	workspace := t.TempDir()
	sharedTempRoot := appfs.RuntimeSharedTempRoot()
	if sharedTempRoot == "" {
		t.Skip("当前平台没有 Unix 共享临时根")
	}
	roots, err := normalizePolicyRoots([]string{workspace, sharedTempRoot})
	if err != nil {
		t.Fatal(err)
	}
	for _, runtimeKind := range []string{"nxs", "claude"} {
		t.Run(runtimeKind, func(t *testing.T) {
			policy := Policy{
				OwnerUserID: "owner-a",
				RuntimeKind: runtimeKind,
				CWD:         workspace,
				ReadRoots:   roots,
				WriteRoots:  roots,
				Generation:  1,
			}
			if violation := inspectToolAccess(policy, sdkhook.Input{
				CWD:      workspace,
				ToolName: "Bash",
				ToolInput: map[string]any{
					"command": "python3 script.py 2>/tmp/wx_err.log; cat /tmp/wx_err.log",
				},
			}); violation != nil {
				t.Fatalf("%s runtime 的共享临时目录重定向不应被 Hook 拦截: %#v", runtimeKind, violation)
			}
		})
	}
}

func TestWorkspacePolicyHookDeniesShellWriteToReadOnlyRoot(t *testing.T) {
	workspace := t.TempDir()
	readOnlyRoot := t.TempDir()
	readRoots, err := normalizePolicyRoots([]string{workspace, readOnlyRoot})
	if err != nil {
		t.Fatal(err)
	}
	writeRoots, err := normalizePolicyRoots([]string{workspace})
	if err != nil {
		t.Fatal(err)
	}
	policy := Policy{
		OwnerUserID: "owner-a",
		RuntimeKind: "nxs",
		CWD:         workspace,
		ReadRoots:   readRoots,
		WriteRoots:  writeRoots,
		Generation:  1,
	}
	violation := inspectToolAccess(policy, sdkhook.Input{
		CWD:      workspace,
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "printf secret > " + filepath.Join(readOnlyRoot, "secret.txt"),
		},
	})
	if violation == nil {
		t.Fatal("shell 重定向不应写入 read-only root")
	}
}

func TestWorkspacePolicyAuditReportsAllow(t *testing.T) {
	workspace := t.TempDir()
	policy := testPolicy(t, workspace)
	callback := workspacePolicyCallback(ModeAudit, policy)
	output, err := callback(context.Background(), sdkhook.Input{
		CWD:       workspace,
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": filepath.Join(filepath.Dir(workspace), "outside")},
	}, "tool-audit")
	if err != nil {
		t.Fatal(err)
	}
	if output.Continue != nil || output.SpecificOutput != nil {
		t.Fatalf("audit decision = %#v", output)
	}
}

func TestWorkspacePolicyHookRunsAfterExistingHooks(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	policy := testPolicy(t, workspace)
	allowHook := func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
		return sdkhook.Output{
			SpecificOutput: &sdkhook.SpecificOutput{
				HookEventName:      sdkhook.EventPreToolUse,
				PermissionDecision: sdkpermission.BehaviorAllow,
			},
		}, nil
	}
	options := withWorkspacePolicyHook(agentclient.Options{
		Hooks: agentclient.HookOptions{
			Matchers: map[sdkhook.Event][]sdkhook.Matcher{
				sdkhook.EventPreToolUse: {{
					Hooks: []sdkhook.Callback{allowHook},
				}},
			},
		},
	}, ModeEnforce, policy)
	matchers := options.Hooks.Matchers[sdkhook.EventPreToolUse]
	if len(matchers) != 2 {
		t.Fatalf("PreToolUse matcher count = %d, want 2", len(matchers))
	}
	output, err := matchers[len(matchers)-1].Hooks[0](context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPreToolUse,
		CWD:       workspace,
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": filepath.Join(outside, "secret.txt")},
	}, "tool-order")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil ||
		output.SpecificOutput.PermissionDecision != sdkpermission.BehaviorDeny {
		t.Fatalf("mandatory policy output = %#v", output)
	}
}

func TestBuildAuditPolicyDoesNotRequireOSIdentity(t *testing.T) {
	workspace := t.TempDir()
	policy, err := buildAuditPolicy(Input{
		OwnerUserID: "owner-a",
		RuntimeKind: "nxs",
		CWD:         workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if policy.Identity.UID != 0 || policy.Identity.PrivateGID != 0 {
		t.Fatalf("audit policy 不应伪造 OS identity: %#v", policy.Identity)
	}
	if sharedTempRoot := appfs.RuntimeSharedTempRoot(); sharedTempRoot != "" {
		if _, err = policy.authorize(filepath.Join(sharedTempRoot, "runtime.log"), true); err != nil {
			t.Fatalf("audit policy 应允许共享临时目录: %v", err)
		}
	}
}

func TestBuildAuditPolicyPreservesMainAgentIdentity(t *testing.T) {
	workspace := t.TempDir()
	policy, err := buildAuditPolicy(Input{
		OwnerUserID: "owner-a",
		RuntimeKind: "nxs",
		CWD:         workspace,
		IsMainAgent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !policy.IsMainAgent {
		t.Fatalf("audit policy 丢失主智能体身份: %#v", policy)
	}
}

func TestApplyMainAgentKeepsHookWithoutLauncher(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("主智能体 enforce 当前只支持 Linux")
	}
	workspace := t.TempDir()
	options, err := Apply(
		context.Background(),
		agentclient.Options{},
		Config{
			Mode:         ModeEnforce,
			LauncherPath: filepath.Join(workspace, "missing-launcher"),
		},
		Input{
			OwnerUserID: "owner-a",
			RuntimeKind: "nxs",
			CWD:         workspace,
			IsMainAgent: true,
		},
	)
	if err != nil {
		t.Fatalf("主智能体 enforce 不应依赖普通 runtime launcher: %v", err)
	}
	matchers := options.Hooks.Matchers[sdkhook.EventPreToolUse]
	if len(matchers) != 1 || len(matchers[0].Hooks) != 1 {
		t.Fatalf("主智能体应保留一个 mandatory workspace hook: %#v", options.Hooks.Matchers)
	}
	output, err := matchers[0].Hooks[0](context.Background(), sdkhook.Input{
		CWD:      workspace,
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": `"$NEXUSCTL_COMMAND_PATH" --json agent list`,
		},
	}, "main-tool")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput != nil {
		t.Fatalf("主智能体 nexusctl 被 hook 拒绝: %#v", output)
	}
}

func TestValidateEnforceOptionsRejectsCallerArguments(t *testing.T) {
	for _, test := range []struct {
		name    string
		options agentclient.Options
	}{
		{
			name: "executable args",
			options: agentclient.Options{
				ExecutableArgs: []string{"--loader"},
			},
		},
		{
			name: "extra args",
			options: agentclient.Options{
				ExtraArgs: map[string]string{"settings": "/tmp/untrusted.json"},
			},
		},
		{
			name: "extra bool args",
			options: agentclient.Options{
				ExtraBoolArgs: []string{"disable-hooks"},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateEnforceOptions(test.options); err == nil {
				t.Fatal("调用方注入的 runtime argv 应被拒绝")
			}
		})
	}
	if err := validateEnforceOptions(agentclient.Options{}); err != nil {
		t.Fatalf("空的受控 options 应被接受: %v", err)
	}
}

func testPolicy(t *testing.T, workspace string) Policy {
	t.Helper()
	readRoots, err := normalizePolicyRoots([]string{workspace})
	if err != nil {
		t.Fatal(err)
	}
	writeRoots, err := normalizePolicyRoots([]string{workspace})
	if err != nil {
		t.Fatal(err)
	}
	return Policy{
		OwnerUserID: "owner-a",
		RuntimeKind: "nxs",
		CWD:         workspace,
		ReadRoots:   readRoots,
		WriteRoots:  writeRoots,
		Generation:  1,
	}
}
