package workspaceisolation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
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
		{name: "relative escape", command: "cat ../../other/secret", denied: true},
		{name: "redirect escape", command: "printf secret > ../../other/secret", denied: true},
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
