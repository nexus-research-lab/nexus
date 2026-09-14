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
	if options.Sandbox == nil || !options.Sandbox.RequireSandbox {
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
				if got.Sandbox != nil {
					t.Fatal("Full Access acquired a sandbox restriction")
				}
				continue
			}
			if got.Sandbox == nil || !got.Sandbox.RequireSandbox || !*got.Sandbox.FailIfUnavailable {
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
