package workspaceisolation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// syscall 只在 Windows 构建导出该名称；数值来自 ERROR_PRIVILEGE_NOT_HELD。
const windowsSymlinkPrivilegeNotHeld syscall.Errno = 1314

func createWorkspaceIsolationTestSymlink(t *testing.T, target string, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" && (errors.Is(err, windowsSymlinkPrivilegeNotHeld) ||
		errors.Is(err, os.ErrPermission) ||
		errors.Is(err, errors.ErrUnsupported)) {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Fatalf("创建测试符号链接失败: %v", err)
}

func TestWorkspacePolicyHookAllowsOwnWorkspaceAndDeniesOtherUser(t *testing.T) {
	root := t.TempDir()
	ownerRoot := filepath.Join(root, "users", "owner-a")
	workspace := filepath.Join(root, "users", "owner-a", "workspace", "agent-a")
	otherWorkspace := filepath.Join(root, "users", "owner-b", "workspace", "agent-b")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	policy := testPolicy(t, ownerRoot)
	policy.CWD = workspace
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

func TestWorkspacePolicyHookAllowsOwnerDataRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	ownerRoot := filepath.Join(root, "users", "owner-a")
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
	createWorkspaceIsolationTestSymlink(t, outsidePath, symlinkSummaryPath)
	policy := testPolicy(t, ownerRoot)
	policy.CWD = workspace
	for _, test := range []struct {
		name     string
		toolName string
		cwd      string
		path     string
		denied   bool
	}{
		{name: "exact Edit", toolName: "Edit", path: summaryPath},
		{
			name:     "relative Edit from session memory cwd",
			toolName: "Edit",
			cwd:      filepath.Dir(summaryPath),
			path:     "summary.md",
		},
		{
			name:     "relative Read from session memory cwd",
			toolName: "Read",
			cwd:      filepath.Dir(summaryPath),
			path:     "summary.md",
		},
		{name: "Write runtime file", toolName: "Write", path: summaryPath},
		{
			name:     "Edit adjacent runtime file",
			toolName: "Edit",
			path:     filepath.Join(filepath.Dir(summaryPath), "state.json"),
		},
		{
			name:     "Write owner state",
			toolName: "Write",
			path:     filepath.Join(ownerRoot, "state", "rooms", "ledger.jsonl"),
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
			cwd := test.cwd
			if cwd == "" {
				cwd = workspace
			}
			violation := inspectToolAccess(policy, sdkhook.Input{
				CWD:      cwd,
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
}

func TestWorkspacePolicyHookAllowsOwnerTranscriptShellAccess(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	ownerRoot := filepath.Join(root, "users", "owner-a")
	workspace := filepath.Join(root, "users", "owner-a", "workspace", "agent-a")
	projectDir := filepath.Join(root, "users", "owner-a", "runtime", "projects", "project-a")
	transcriptPath := filepath.Join(projectDir, "session-a.jsonl")
	siblingPath := filepath.Join(projectDir, "session-b.jsonl")
	otherOwnerPath := filepath.Join(root, "users", "owner-b", "runtime", "projects", "project-b", "session.jsonl")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{transcriptPath, siblingPath, otherOwnerPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("transcript\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	policy := testPolicy(t, ownerRoot)
	policy.CWD = workspace

	for _, test := range []struct {
		name      string
		toolName  string
		toolInput map[string]any
		denied    bool
	}{
		{
			name:      "Read current transcript",
			toolName:  "Read",
			toolInput: map[string]any{"file_path": transcriptPath},
		},
		{
			name:      "Grep current transcript",
			toolName:  "Grep",
			toolInput: map[string]any{"path": transcriptPath, "pattern": "needle"},
		},
		{
			name:      "Read sibling transcript",
			toolName:  "Read",
			toolInput: map[string]any{"file_path": siblingPath},
		},
		{
			name:      "Bash current transcript",
			toolName:  "Bash",
			toolInput: map[string]any{"command": "grep needle " + transcriptPath},
		},
		{
			name:      "Bash scan owner transcript directory",
			toolName:  "Bash",
			toolInput: map[string]any{"command": "d=\"" + projectDir + "\"; ls -lat \"$d\"; grep -l needle \"$d\"/*.jsonl"},
		},
		{
			name:      "Write owner transcript",
			toolName:  "Write",
			toolInput: map[string]any{"file_path": transcriptPath},
		},
		{
			name:      "Bash overwrite owner transcript",
			toolName:  "Bash",
			toolInput: map[string]any{"command": "printf changed > \"" + transcriptPath + "\""},
		},
		{
			name:      "Bash remove owner transcript",
			toolName:  "Bash",
			toolInput: map[string]any{"command": "rm \"" + transcriptPath + "\""},
		},
		{
			name:      "Read other owner transcript",
			toolName:  "Read",
			toolInput: map[string]any{"file_path": otherOwnerPath},
			denied:    true,
		},
		{
			name:      "Bash scan other owner transcript directory",
			toolName:  "Bash",
			toolInput: map[string]any{"command": "d=\"" + filepath.Dir(otherOwnerPath) + "\"; ls -lat \"$d\"; grep -l needle \"$d\"/*.jsonl"},
			denied:    true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			violation := inspectToolAccess(policy, sdkhook.Input{
				CWD:       workspace,
				ToolName:  test.toolName,
				ToolInput: test.toolInput,
			})
			if (violation != nil) != test.denied {
				t.Fatalf("violation = %#v, denied=%v", violation, test.denied)
			}
		})
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
	createWorkspaceIsolationTestSymlink(t, otherWorkspace, filepath.Join(workspace, "escape"))
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
		{name: "workspace absolute path", command: "type " + filepath.Join(workspace, "README.md"), denied: false},
		{
			name:    "quoted workspace path with spaces",
			command: `type "` + filepath.Join(workspace, "folder with spaces", "README.md") + `"`,
		},
		{
			name:    "quoted outside path with spaces",
			command: `type "` + filepath.Join(filepath.Dir(workspace), "outside folder", "secret.txt") + `"`,
			denied:  true,
		},
		{name: "nexusctl filename substring", command: "type ./Nexusctl-not-command.txt", denied: false},
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
		{name: "dynamic braced workspace path", command: "cat ./logs/${name}", denied: false},
		{name: "url braced default", command: `curl "${BASE_URL:-https://example.com}/health"`, denied: false},
		{name: "url option braced default", command: `curl --url="${BASE_URL:-https://example.com}/health"`, denied: false},
		{name: "local file URL", command: `curl file:///home/other/secret`, denied: true},
		{name: "windows local file URL", command: `curl file:///C:/Users/other/secret`, denied: true},
		{name: "windows local file URL query", command: `curl file:///C:/Windows/win.ini?x=y`, denied: true},
		{name: "windows local file URL option", command: `curl --url=file:///C:/Windows/win.ini?x=y`, denied: true},
		{name: "braced local file URL", command: `curl "${URL:-file:///home/other/secret}"`, denied: true},
		{name: "mime braced default", command: "echo ${MIME:-text/plain}", denied: false},
		{
			name:    "dynamic outside prefix",
			command: "cat " + filepath.Join(filepath.Dir(workspace), "outside", "$name"),
			denied:  true,
		},
		{
			name:    "dynamic braced outside prefix",
			command: "cat " + filepath.Join(filepath.Dir(workspace), "outside", "${name}"),
			denied:  true,
		},
		{name: "relative escape", command: "cat ../../other/secret", denied: true},
		{name: "redirect escape", command: "printf secret > ../../other/secret", denied: true},
		{name: "home shorthand", command: "cat ~/secret", denied: true},
		{name: "named home shorthand", command: "cat ~other/secret", denied: true},
		{name: "shell variable", command: "cat $HOME/secret", denied: true},
		{name: "braced shell variable", command: "cat ${HOME}/secret", denied: true},
		{name: "non-path braced default", command: "echo ${name:-default}", denied: false},
		{name: "complex braced workspace path", command: "cat ./logs/${name:-default}", denied: true},
		{name: "complex braced traversal", command: "cat ./logs/${name:-../../outside}/secret", denied: true},
		{name: "url marker local traversal", command: `cat "./logs/${name:-../../outside}/secret://suffix"`, denied: true},
		{name: "complex braced drive relative", command: "cat ${name:-C:secret}", denied: true},
		{name: "braced replacement root", command: `name=x; cat "${name/#x//etc/passwd}"`, denied: true},
		{name: "nested braced path", command: "cat ./logs/${name:-${fallback}/child}", denied: true},
		{name: "cmd variable", command: `type %USERPROFILE%\secret`, denied: true},
		{name: "windows absolute path", command: `type C:\Users\other\secret`, denied: true},
		{name: "windows drive relative path", command: `type C:secret`, denied: true},
		{name: "windows drive only path", command: `type C:`, denied: true},
		{name: "dynamic windows drive relative path", command: `type C:$name`, denied: true},
		{name: "windows rooted path", command: `type \Windows\secret`, denied: runtime.GOOS == "windows"},
		{name: "short windows rooted path", command: `type \a`, denied: runtime.GOOS == "windows"},
		{name: "numeric windows rooted path", command: `type \0`, denied: runtime.GOOS == "windows"},
		{name: "quoted numeric windows rooted path", command: `type "\0"`, denied: runtime.GOOS == "windows"},
		{name: "numeric windows rooted path nine", command: `type '\9'`, denied: runtime.GOOS == "windows"},
		{name: "windows root path", command: `type \`, denied: runtime.GOOS == "windows"},
		{name: "bash escaped backslash traversal", command: `type ..\..\other\secret`, denied: false},
		{
			name:    "outside path containing equals",
			command: "type " + filepath.Join(filepath.Dir(workspace), "outside", "file=name"),
			denied:  true,
		},
		{name: "nexusctl broker pending", command: "nexusctl agent list", denied: true},
		{name: "scoped nexuscfg", command: "nexuscfg inspect", denied: false},
		{name: "scoped nexus runtime cli", command: `"${NEXUS_COMMAND_PATH}" --json automation contract`, denied: false},
		{name: "concatenated nexusctl", command: `nex"usctl" agent list`, denied: true},
		{name: "escaped nexusctl", command: `nex\usctl agent list`, denied: true},
		{name: "cmd escaped nexusctl", command: `nex^usctl agent list`, denied: true},
		{name: "powershell escaped nexusctl", command: "nex`usctl agent list", denied: true},
		{name: "ansi quoted nexusctl", command: `$'nexusctl' agent list`, denied: true},
		{name: "ansi concatenated nexusctl", command: `nex$'usctl' agent list`, denied: true},
		{name: "windows nexusctl shim", command: `C:\tools\nexusctl.cmd agent list`, denied: true},
		{name: "powershell command path", command: `& $env:NEXUSCTL_COMMAND_PATH agent list`, denied: true},
		{name: "braced command path", command: `"${NEXUSCTL_COMMAND_PATH}" agent list`, denied: true},
		{name: "braced nexuscfg command path", command: `"${NEXUSCFG_COMMAND_PATH}" inspect`, denied: false},
		{name: "forged nexuscfg broker", command: `NEXUSCFG_BROKER_URL=http://127.0.0.1:9 nexuscfg inspect`, denied: true},
		{name: "forged nexus command broker", command: `NEXUS_COMMAND_BROKER_URL=http://127.0.0.1:9 nexus automation contract`, denied: true},
		{name: "forged nexus command capability", command: `NEXUS_COMMAND_CAPABILITY_TOKEN=fake nexus automation contract`, denied: true},
		{name: "braced powershell command path", command: `& ${env:NEXUSCTL_COMMAND_PATH} agent list`, denied: true},
		{name: "assigned nexusctl", command: `cmd=nexusctl; "$cmd" agent list`, denied: true},
		{name: "assigned command path", command: `cmd="$NEXUSCTL_COMMAND_PATH"; "$cmd" agent list`, denied: true},
		{name: "nested bash nexusctl", command: `bash -c 'nexusctl agent list'`, denied: true},
		{name: "nested login bash nexusctl", command: `bash -lc 'nexusctl agent list'`, denied: true},
		{name: "nested cmd nexusctl", command: `cmd /c "nexusctl agent list"`, denied: true},
		{
			name:    "encoded powershell command",
			command: `powershell -EncodedCommand RwBlAHQALQBDAG8AbgB0AGUAbgB0ACAAQwA6AFwAVwBpAG4AZABvAHcAcwBcAHcAaQBuAC4AaQBuAGkA`,
			denied:  true,
		},
		{name: "abbreviated encoded pwsh command", command: `pwsh -enc RwBlAHQALQBEAGEAdABlAA==`, denied: true},
		{
			name:    "nested bash outside path",
			command: `bash -c 'cat ` + filepath.Join(filepath.Dir(workspace), "outside", "secret.txt") + `'`,
			denied:  true,
		},
		{
			name:    "nested clustered sh outside path",
			command: `sh -ec 'cat ` + filepath.Join(filepath.Dir(workspace), "outside", "secret.txt") + `'`,
			denied:  true,
		},
		{
			name:    "escaped quote before redirect",
			command: `printf '%s' "\""; printf secret > ` + filepath.Join(filepath.Dir(workspace), "outside", "secret.txt"),
			denied:  true,
		},
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

func TestWorkspacePolicyHookAllowsMainAgentControlCLIs(t *testing.T) {
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
			name:    "injected nexuscfg command path",
			command: `"$NEXUSCFG_COMMAND_PATH" --json inspect --domain providers`,
		},
		{
			name:    "bare nexuscfg command path",
			command: "nexuscfg --json inspect --domain providers",
		},
		{
			name:    "injected nexus runtime command path",
			command: `"$NEXUS_COMMAND_PATH" --json automation contract`,
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
			name:    "quoted environment override",
			command: "env 'NEXUSCTL_USER_ID=owner-b' nexusctl agent list",
			denied:  true,
		},
		{
			name:    "concatenated global scope",
			command: `nexusctl --global"-scope" agent list`,
			denied:  true,
		},
		{
			name:    "assigned command scope override",
			command: `cmd=nexusctl; "$cmd" --global-scope agent list`,
			denied:  true,
		},
		{
			name:    "assigned flag scope override",
			command: `flag=--global-scope; nexusctl "$flag" agent list`,
			denied:  true,
		},
		{
			name:    "powershell environment override",
			command: `$env:NEXUSCTL_USER_ID='owner-b'; nexusctl agent list`,
			denied:  true,
		},
		{
			name:    "runtime scope mode override",
			command: `NEXUS_RUNTIME_SCOPE_MODE= nexusctl agent list`,
			denied:  true,
		},
		{
			name:    "braced powershell scope mode override",
			command: `${env:NEXUS_RUNTIME_SCOPE_MODE}=''; nexusctl agent list`,
			denied:  true,
		},
		{
			name:    "powershell set environment override",
			command: `Set-Item Env:NEXUSCTL_USER_ID owner-b; nexusctl agent list`,
			denied:  true,
		},
		{
			name:    "multiline powershell environment override",
			command: "Write-Output ok\nSet-Item Env:NEXUSCTL_USER_ID owner-b\nnexusctl agent list",
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
	for _, mode := range []Mode{ModeAudit, ModeEnforce} {
		t.Run(string(mode), func(t *testing.T) {
			callback := workspacePolicyCallback(mode, testPolicy(t, workspace))
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
		})
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
	tests := []struct {
		name    string
		command string
		denied  bool
	}{
		{
			name:    "unquoted read-only target",
			command: "printf secret > " + filepath.Join(readOnlyRoot, "secret.txt"),
			denied:  true,
		},
		{
			name:    "quoted read-only target",
			command: `printf secret > "` + filepath.Join(readOnlyRoot, "folder with spaces", "secret.txt") + `"`,
			denied:  true,
		},
		{
			name:    "adjacent quoted read-only target",
			command: `printf secret>"` + filepath.Join(readOnlyRoot, "folder with spaces", "secret.txt") + `"`,
			denied:  true,
		},
		{
			name:    "dynamic read-only target",
			command: `printf secret > "` + filepath.Join(readOnlyRoot, "folder with spaces", "$name") + `"`,
			denied:  true,
		},
		{
			name:    "single quoted read-only target",
			command: `printf secret > '` + filepath.Join(readOnlyRoot, "folder with spaces", "secret.txt") + `'`,
			denied:  true,
		},
		{
			name:    "noclobber override read-only target",
			command: `printf secret >| "` + filepath.Join(readOnlyRoot, "folder with spaces", "secret.txt") + `"`,
			denied:  true,
		},
		{
			name:    "quoted writable target",
			command: `printf secret > "` + filepath.Join(workspace, "folder with spaces", "secret.txt") + `"`,
		},
		{
			name:    "dynamic writable target",
			command: `printf secret > "` + filepath.Join(workspace, "folder with spaces", "$name") + `"`,
		},
		{
			name:    "bare dynamic target",
			command: `printf secret > "$LOG_PATH"`,
		},
		{
			name:    "quoted redirection text",
			command: `printf '%s' "> ` + filepath.Join(readOnlyRoot, "folder with spaces", "secret.txt") + `"`,
		},
		{
			name:    "file descriptor duplication",
			command: "printf secret 2>&1",
		},
		{
			name:    "escaped redirection operator",
			command: `printf '%s' \> ` + filepath.Join(readOnlyRoot, "secret.txt"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violation := inspectToolAccess(policy, sdkhook.Input{
				CWD:       workspace,
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": test.command},
			})
			if got := violation != nil; got != test.denied {
				t.Fatalf("violation = %#v, denied = %v", violation, test.denied)
			}
		})
	}
	for _, test := range []struct {
		toolName string
		command  string
	}{
		{
			toolName: "PowerShell",
			command:  "Write-Output `> " + filepath.Join(readOnlyRoot, "secret.txt"),
		},
		{
			toolName: "Shell",
			command:  "echo ^> " + filepath.Join(readOnlyRoot, "secret.txt"),
		},
	} {
		if test.toolName == "Shell" && runtime.GOOS != "windows" {
			continue
		}
		if violation := inspectToolAccess(policy, sdkhook.Input{
			CWD:       workspace,
			ToolName:  test.toolName,
			ToolInput: map[string]any{"command": test.command},
		}); violation != nil {
			t.Fatalf("%s escaped redirection should be allowed: %#v", test.toolName, violation)
		}
	}
	if violation := inspectToolAccess(policy, sdkhook.Input{
		CWD:      workspace,
		ToolName: "PowerShell",
		ToolInput: map[string]any{
			"command": "Write-Output '`' > " + filepath.Join(readOnlyRoot, "secret.txt"),
		},
	}); violation == nil {
		t.Fatal("PowerShell 单引号内的 backtick 不应吞掉真实重定向")
	}
	mainPolicy := policy
	mainPolicy.IsMainAgent = true
	if violation := inspectToolAccess(mainPolicy, sdkhook.Input{
		CWD:       workspace,
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": `printf secret > "$LOG_PATH"`},
	}); violation == nil {
		t.Fatal("主智能体不应写入无法静态授权的动态重定向目标")
	}
	for _, test := range []struct {
		toolName string
		command  string
	}{
		{toolName: "Bash", command: `name=$'..'; printf secret > "` + filepath.Join(workspace, "$name", "secret.txt") + `"`},
		{toolName: "Bash", command: `name=$'..'; cat "` + filepath.Join(workspace, "$name", "secret.txt") + `"`},
		{toolName: "PowerShell", command: `Resolve-Path $env:ComSpec`},
		{toolName: "PowerShell", command: `Resolve-Path ${env:ComSpec}`},
		{toolName: "Bash", command: `cat "$HOME"`},
		{toolName: "PowerShell", command: `Get-Item $env:windir`},
		{toolName: "Shell", command: `type %USERPROFILE%`},
	} {
		if violation := inspectToolAccess(mainPolicy, sdkhook.Input{
			CWD:       workspace,
			ToolName:  test.toolName,
			ToolInput: map[string]any{"command": test.command},
		}); violation == nil {
			t.Fatalf("主智能体不应使用无法静态收口的动态路径: %q", test.command)
		}
	}
}

func TestWorkspacePolicyHookRejectsPowerShellSlashRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell /path 仅在 Windows 表示当前盘根路径")
	}
	workspace := t.TempDir()
	policy := testPolicy(t, workspace)
	for _, command := range []string{
		`Resolve-Path /Windows`,
		`Get-Content "/Windows/secret.txt"`,
		"Get-Content `/Windows/secret.txt",
		`Get-Content FileSystem::C:\Users\other\secret.txt`,
		`Get-Item -LiteralPath:C:\Windows`,
		`Get-Item -Path:FileSystem::C:\Windows`,
		`Get-Acl /Windows`,
		`Get-FileHash /secret.txt`,
		`Import-Csv /secret.csv`,
		`python -c "print('x')" /secret.txt`,
		"cmd /c echo ok\nGet-Content /c",
		`printf secret > "/Windows/secret.txt"`,
	} {
		if violation := inspectToolAccess(policy, sdkhook.Input{
			CWD:       workspace,
			ToolName:  "PowerShell",
			ToolInput: map[string]any{"command": command},
		}); violation == nil {
			t.Fatalf("PowerShell current-drive root path should be denied: %q", command)
		}
	}
	for _, command := range []string{
		`cmd /c echo ok`,
		`findstr /i pattern file.txt`,
		`ping /?`,
	} {
		if violation := inspectToolAccess(policy, sdkhook.Input{
			CWD:       workspace,
			ToolName:  "PowerShell",
			ToolInput: map[string]any{"command": command},
		}); violation != nil {
			t.Fatalf("PowerShell slash option should be allowed: %q: %#v", command, violation)
		}
	}
}

func TestWorkspacePolicyHookRejectsWindowsBackslashTraversal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("反斜杠路径穿越仅由 Windows Shell 解释为目录穿越")
	}
	workspace := t.TempDir()
	policy := testPolicy(t, workspace)
	for _, test := range []struct {
		toolName string
		command  string
	}{
		{toolName: "Shell", command: `type ..\..\other\secret`},
		{toolName: "PowerShell", command: `Get-Content ..\..\other\secret`},
	} {
		if violation := inspectToolAccess(policy, sdkhook.Input{
			CWD:       workspace,
			ToolName:  test.toolName,
			ToolInput: map[string]any{"command": test.command},
		}); violation == nil {
			t.Fatalf("%s 应拒绝 Windows 反斜杠路径穿越: %q", test.toolName, test.command)
		}
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
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
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
	if _, err = policy.authorize(
		filepath.Join(appfs.UserStateRoot("owner-a"), "rooms", "ledger.jsonl"),
		true,
	); err != nil {
		t.Fatalf("audit policy 应允许当前 owner 数据根: %v", err)
	}
	if _, err = policy.authorize(appfs.UserDataRoot("owner-b"), false); err == nil {
		t.Fatal("audit policy 不应允许其他 owner 数据根")
	}
	if sharedTempRoot := appfs.RuntimeSharedTempRoot(); sharedTempRoot != "" {
		if _, err = policy.authorize(filepath.Join(sharedTempRoot, "runtime.log"), true); err != nil {
			t.Fatalf("audit policy 应允许共享临时目录: %v", err)
		}
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
