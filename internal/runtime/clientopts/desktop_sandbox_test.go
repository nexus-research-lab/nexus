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

func TestBuildAgentClientOptionsInstallsDesktopPolicy(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: t.TempDir(), RuntimeKind: "nxs", AppMode: "desktop", DesktopSandboxEnabled: true,
		PermissionMode: sdkpermission.ModeDefault, AdditionalDirectories: []string{t.TempDir()},
	})
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		if err == nil {
			t.Fatal("unsupported desktop platform accepted")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if options.Sandbox == nil || !options.Sandbox.RequireSandbox || !options.Sandbox.RequireFileTools || !options.Sandbox.RequireSearchTools || !options.Sandbox.RequireMediaFiles || !options.Sandbox.RequireSkillFiles || !options.Sandbox.RequireContextFiles || !options.Sandbox.RequireProjectFiles || !options.Sandbox.RequireManagedPolicy || !options.Sandbox.RequireSettingsFiles || !options.Sandbox.RequireSettingsWrites {
		t.Fatal("common builder dropped desktop sandbox policy")
	}
}

func TestDesktopSandboxPolicySeparatesResourcesAndFullAccess(t *testing.T) {
	input := AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true,
		SkillDirectories: []string{"/skills"}, AdditionalDirectories: []string{"/mounted"}}
	for _, platform := range []string{"darwin", "windows"} {
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
			if got.Sandbox == nil || !got.Sandbox.RequireSandbox || !got.Sandbox.RequireFileTools || !got.Sandbox.RequireSearchTools || !got.Sandbox.RequireMediaFiles || !got.Sandbox.RequireSkillFiles || !got.Sandbox.RequireContextFiles || !got.Sandbox.RequireProjectFiles || !got.Sandbox.RequireManagedPolicy || !got.Sandbox.RequireSettingsFiles || !got.Sandbox.RequireSettingsWrites || !*got.Sandbox.FailIfUnavailable {
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
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}}, input, "darwin"); err == nil {
		t.Fatal("unsupported runtime silently accepted")
	}
}
