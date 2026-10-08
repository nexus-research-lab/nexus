package clientopts

import (
	"context"
	"reflect"
	"runtime"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildAgentClientOptionsInstallsClaudeNativeContract(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: t.TempDir(), RuntimeKind: "claude", AppMode: "desktop", PermissionMode: sdkpermission.ModeDefault,
	})
	if runtime.GOOS != "darwin" {
		if err == nil {
			t.Fatal("unsupported Claude native sandbox was accepted")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if options.Sandbox == nil || !options.Sandbox.RequireClaudeNativeSandbox {
		t.Fatalf("Claude native sandbox contract missing: %#v", options.Sandbox)
	}
	if options.Sandbox.RequireClaudeRestricted || options.Sandbox.RequireSandbox || options.Sandbox.RequireFileTools || options.Sandbox.RequireSearchTools || options.Sandbox.Enabled == nil || !*options.Sandbox.Enabled || options.Sandbox.FailIfUnavailable == nil || !*options.Sandbox.FailIfUnavailable || options.Sandbox.AllowUnsandboxedCommands == nil || *options.Sandbox.AllowUnsandboxedCommands {
		t.Fatalf("Claude claimed nxs sandbox capabilities: %#v", options.Sandbox)
	}
}

func TestDesktopSandboxPolicySeparatesResourcesAndFullAccess(t *testing.T) {
	input := AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true,
		SkillDirectories: []string{"/skills"}, AdditionalDirectories: []string{"/mounted"}}
	for _, platform := range []string{"darwin"} {
		for _, mode := range []sdkpermission.Mode{sdkpermission.ModeDefault, sdkpermission.ModeAuto, sdkpermission.ModeBypassPermissions} {
			before := agentclient.Options{Env: map[string]string{"existing": "value"}, Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS, PermissionMode: mode}}
			got, err := applyDesktopSandboxForPlatform(before, input, platform)
			if err != nil {
				t.Fatal(err)
			}
			if got.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "1" || before.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "" {
				t.Fatal("host policy marker missing or caller map mutated")
			}
			if got.Runtime.PermissionMode != mode {
				t.Fatal("sandbox changed approval mode")
			}
			if mode == sdkpermission.ModeBypassPermissions {
				if got.Sandbox == nil || !got.Sandbox.RequireSandbox || !got.Sandbox.RequireFileTools || !got.Sandbox.RequireSettingsWrites || got.Sandbox.AllowUnsandboxedCommands == nil || !*got.Sandbox.AllowUnsandboxedCommands {
					t.Fatal("Full Access dropped the nxs runtime boundary")
				}
				continue
			}
			if got.Sandbox == nil || !got.Sandbox.RequireSandbox || !got.Sandbox.RequireFileTools || !got.Sandbox.RequireSearchTools || !got.Sandbox.RequireMediaFiles || !got.Sandbox.RequireMediaNetwork || !got.Sandbox.RequireNotebookFiles || !got.Sandbox.RequireSkillFiles || !got.Sandbox.RequireContextFiles || !got.Sandbox.RequireProjectFiles || !got.Sandbox.RequireManagedPolicy || !got.Sandbox.RequireSettingsFiles || !got.Sandbox.RequireSettingsWrites || !*got.Sandbox.FailIfUnavailable {
				t.Fatal("mandatory execution requirement lost")
			}
			if !reflect.DeepEqual(got.Sandbox.Filesystem.AllowRead, input.SkillDirectories) || !reflect.DeepEqual(got.Sandbox.Filesystem.AllowWrite, input.AdditionalDirectories) {
				t.Fatal("resource/write grants were conflated")
			}
			got.Sandbox.Filesystem.AllowWrite[0] = "/changed"
			if input.AdditionalDirectories[0] != "/mounted" {
				t.Fatal("policy aliases caller grants")
			}
		}
	}
}

func TestDesktopSandboxUsesClaudeNativeCommandSandboxContract(t *testing.T) {
	input := AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}
	for _, platform := range []string{"darwin"} {
		for _, mode := range []sdkpermission.Mode{sdkpermission.ModeDefault, sdkpermission.ModeAuto, sdkpermission.ModeAcceptEdits} {
			got, err := applyDesktopSandboxForPlatform(agentclient.Options{
				Env:     map[string]string{"existing": "value"},
				Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude, PermissionMode: mode},
			}, input, platform)
			if err != nil {
				t.Fatalf("platform=%s mode=%s: %v", platform, mode, err)
			}
			if got.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "1" {
				t.Fatalf("platform=%s mode=%s: host policy marker missing", platform, mode)
			}
			if got.Sandbox == nil || !got.Sandbox.RequireClaudeNativeSandbox {
				t.Fatalf("platform=%s mode=%s: Claude native sandbox contract missing: %#v", platform, mode, got.Sandbox)
			}
			if got.Sandbox.RequireClaudeRestricted || got.Sandbox.RequireSandbox || got.Sandbox.RequireFileTools || got.Sandbox.Resources != nil || got.Sandbox.Enabled == nil || !*got.Sandbox.Enabled || got.Sandbox.FailIfUnavailable == nil || !*got.Sandbox.FailIfUnavailable || got.Sandbox.AllowUnsandboxedCommands == nil || *got.Sandbox.AllowUnsandboxedCommands {
				t.Fatalf("platform=%s mode=%s: Claude claimed nxs sandbox contract: %#v", platform, mode, got.Sandbox)
			}
		}
	}
	gotWindows, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}}, input, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if gotWindows.Sandbox != nil || gotWindows.Env != nil {
		t.Fatalf("Windows must retain the unsandboxed Claude contract: %#v", gotWindows)
	}
}

func TestDesktopSandboxClaudeFullAccessDoesNotInstallRestrictedContract(t *testing.T) {
	got, err := applyDesktopSandboxForPlatform(agentclient.Options{
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude, PermissionMode: sdkpermission.ModeBypassPermissions},
	}, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Sandbox != nil && got.Sandbox.RequireClaudeRestricted {
		t.Fatalf("Claude Full Access installed restricted contract: %#v", got.Sandbox)
	}
	if got.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "1" {
		t.Fatal("Claude Full Access lost the host lifecycle marker")
	}
}

func TestDesktopSandboxClaudeRestrictedPreservesOrdinarySettingsButRejectsNXSContract(t *testing.T) {
	enabled := true
	ordinary := &agentclient.SandboxSettings{Enabled: &enabled, Extra: map[string]any{"uiHint": "native"}}
	got, err := applyDesktopSandboxForPlatform(agentclient.Options{
		Sandbox: ordinary,
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude, PermissionMode: sdkpermission.ModeDefault},
	}, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Sandbox == ordinary || got.Sandbox == nil || !got.Sandbox.RequireClaudeNativeSandbox || got.Sandbox.RequireClaudeRestricted || got.Sandbox.Enabled == nil || !*got.Sandbox.Enabled || got.Sandbox.FailIfUnavailable == nil || !*got.Sandbox.FailIfUnavailable || got.Sandbox.AllowUnsandboxedCommands == nil || *got.Sandbox.AllowUnsandboxedCommands || got.Sandbox.Extra["uiHint"] != "native" {
		t.Fatalf("ordinary Claude settings were not copied: %#v", got.Sandbox)
	}
	ordinary.RequireClaudeNativeSandbox = false
	if got.Sandbox.RequireClaudeNativeSandbox != true {
		t.Fatal("Claude native sandbox contract aliases caller settings")
	}
	for _, existing := range []*agentclient.SandboxSettings{{RequireSandbox: true}, {RequireFileTools: true}, {Resources: &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeReadOnly, ScratchRoot: "/tmp/scratch"}}} {
		if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Sandbox: existing, Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}}, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin"); err == nil {
			t.Fatalf("mixed nxs Claude sandbox contract accepted: %#v", existing)
		}
	}
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}}, AgentClientOptionsInput{
		AppMode:               "desktop",
		DesktopSandboxEnabled: true,
		SandboxResources: &agentclient.SandboxResourcePolicy{
			Version: 1, WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: "/tmp/scratch",
		},
	}, "darwin"); err == nil {
		t.Fatal("Claude native sandbox accepted an nxs host resource contract")
	}
	gotWindows, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude, PermissionMode: sdkpermission.ModeBypassPermissions}}, AgentClientOptionsInput{
		AppMode:               "desktop",
		DesktopSandboxEnabled: true,
		SandboxResources: &agentclient.SandboxResourcePolicy{
			Version: 1, WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: "/tmp/scratch",
		},
	}, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if gotWindows.Sandbox != nil {
		t.Fatalf("Windows must defer the desktop sandbox contract: %#v", gotWindows.Sandbox)
	}
}

func TestDesktopSandboxCopiesHostPreparedResources(t *testing.T) {
	scratchRoot := t.TempDir()
	input := AgentClientOptionsInput{
		AppMode:               "desktop",
		DesktopSandboxEnabled: true,
		SandboxResources: &agentclient.SandboxResourcePolicy{
			Version:     1,
			WriteScope:  agentclient.SandboxWriteScopeWorkspaceWrite,
			ScratchRoot: scratchRoot,
		},
	}
	for _, mode := range []sdkpermission.Mode{sdkpermission.ModeDefault, sdkpermission.ModeAuto} {
		before := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS, PermissionMode: mode}}
		got, err := applyDesktopSandboxForPlatform(before, input, "darwin")
		if err != nil {
			t.Fatalf("mode %s: %v", mode, err)
		}
		if got.Sandbox == nil || got.Sandbox.Resources == nil {
			t.Fatalf("mode %s: resources missing", mode)
		}
		if got.Sandbox.Resources == input.SandboxResources {
			t.Fatalf("mode %s: resource policy aliases caller", mode)
		}
		if *got.Sandbox.Resources != *input.SandboxResources {
			t.Fatalf("mode %s: resources = %#v, want %#v", mode, got.Sandbox.Resources, input.SandboxResources)
		}
		if got.Sandbox.AllowUnsandboxedCommands == nil || *got.Sandbox.AllowUnsandboxedCommands {
			t.Fatalf("mode %s: resource policy left unsandboxed commands enabled", mode)
		}
		got.Sandbox.Resources.ScratchRoot = "/tmp/changed"
		if input.SandboxResources.ScratchRoot != scratchRoot {
			t.Fatalf("mode %s: caller resource policy mutated", mode)
		}
	}
}

func TestDesktopSandboxRejectsInvalidOrFullAccessResources(t *testing.T) {
	invalid := AgentClientOptionsInput{
		AppMode:               "desktop",
		DesktopSandboxEnabled: true,
		SandboxResources:      &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeReadOnly, ScratchRoot: "relative"},
	}
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS, PermissionMode: sdkpermission.ModeDefault}}, invalid, "darwin"); err == nil {
		t.Fatal("invalid host resources accepted")
	}
	fullAccess := invalid
	fullAccess.SandboxResources = &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeReadOnly, ScratchRoot: "/tmp/nexus-sandbox/session-b"}
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS, PermissionMode: sdkpermission.ModeBypassPermissions}}, fullAccess, "darwin"); err == nil {
		t.Fatal("Full Access silently accepted a restricted resource policy")
	}
	readOnlyWithWriteGrant := AgentClientOptionsInput{
		AppMode:               "desktop",
		DesktopSandboxEnabled: true,
		AdditionalDirectories: []string{"/tmp/mounted"},
		SandboxResources: &agentclient.SandboxResourcePolicy{
			Version: 1, WriteScope: agentclient.SandboxWriteScopeReadOnly, ScratchRoot: "/tmp/nexus-sandbox/session-c",
		},
	}
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS, PermissionMode: sdkpermission.ModeDefault}}, readOnlyWithWriteGrant, "darwin"); err == nil {
		t.Fatal("read-only resource policy accepted an explicit write grant")
	}
}

func TestDesktopSandboxDoesNotAlterServerIsolationOrDisabledFeature(t *testing.T) {
	for _, input := range []AgentClientOptionsInput{{AppMode: "server", DesktopSandboxEnabled: true}, {AppMode: "desktop"}} {
		original := &agentclient.SandboxSettings{}
		before := agentclient.Options{Sandbox: original, Env: map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"}}
		got, err := applyDesktopSandboxForPlatform(before, input, "linux")
		if err != nil || got.Sandbox != original || got.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "" {
			t.Fatalf("legacy/server policy changed: %#v, %v", got, err)
		}
	}
	input := AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{}, input, "linux"); err == nil {
		t.Fatal("unsupported desktop platform silently accepted")
	}
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeKind("other")}}, input, "darwin"); err == nil {
		t.Fatal("unsupported runtime silently accepted")
	}
}
