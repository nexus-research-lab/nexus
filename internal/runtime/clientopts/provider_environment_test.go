// INPUT: 后端选择与附加 runtime 环境覆盖。
// OUTPUT: nxs Provider/后台唤醒所有权不能被调用方环境撤销。
// POS: Nexus 到 nxs 的可信启动声明回归，不推断 Claude 原生凭据隔离。
package clientopts

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildAgentClientOptionsPreservesHostProviderOwnership(t *testing.T) {
	for _, source := range []string{"extra", "configuration"} {
		t.Run(source, func(t *testing.T) {
			input := AgentClientOptionsInput{RuntimeKind: runtimeKindNXS, WorkspacePath: t.TempDir()}
			override := map[string]string{nexusProviderManagedByHostEnvName: "0", nexusAutoDreamWakeModeEnvName: "task"}
			if source == "extra" {
				input.ExtraEnv = override
			} else {
				input.ConfigurationEnv = map[string]string{
					protocol.NexusConfigBrokerURLEnvName:       "http://127.0.0.1:8010/configuration",
					protocol.NexusConfigCapabilityTokenEnvName: "test-capability",
				}
			}
			options, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{}, input)
			if err != nil {
				t.Fatal(err)
			}
			if options.Env[nexusProviderManagedByHostEnvName] != "1" || options.Env[nexusAutoDreamWakeModeEnvName] != "host" {
				t.Fatal("runtime environment revoked host provider or wake ownership")
			}
		})
	}
}

func TestBuildClaudeOptionsDoNotClaimNXSProviderOwnership(t *testing.T) {
	options, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{RuntimeKind: runtimeKindClaude, WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if options.Env[nexusProviderManagedByHostEnvName] == "1" || options.Env[nexusAutoDreamWakeModeEnvName] == "host" {
		t.Fatal("Claude received an nxs-specific ownership contract")
	}
}

func TestBuildAgentClientOptionsProtectsMemoryRootFromConfigurationEnv(t *testing.T) {
	workspace := t.TempDir()
	options, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:   runtimeKindNXS,
		WorkspacePath: workspace,
		ExtraEnv: map[string]string{
			nexusMemoryDirEnvName:          "/tmp/escaped-memory",
			nexusEnableRemoteMemoryEnvName: "1",
			nexusRemoteMemoryDirEnvName:    "/tmp/escaped-remote-memory",
		},
		ConfigurationEnv: map[string]string{
			protocol.NexusConfigBrokerURLEnvName:       "http://127.0.0.1:8010/configuration",
			protocol.NexusConfigCapabilityTokenEnvName: "test-capability",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.Env[nexusMemoryDirEnvName] != workspace ||
		options.Env[nexusEnableRemoteMemoryEnvName] != "" ||
		options.Env[nexusRemoteMemoryDirEnvName] != "" {
		t.Fatalf("configuration environment redirected managed memory: %#v", options.Env)
	}
}

func TestBuildAgentClientOptionsRejectsUnknownConfigurationEnvironment(t *testing.T) {
	_, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:   runtimeKindNXS,
		WorkspacePath: t.TempDir(),
		ConfigurationEnv: map[string]string{
			protocol.NexusConfigBrokerURLEnvName:       "http://127.0.0.1:8010/configuration",
			protocol.NexusConfigCapabilityTokenEnvName: "test-capability",
			"HOME": "/tmp/escaped-home",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported environment key") {
		t.Fatalf("unknown nexuscfg environment key was accepted: %v", err)
	}
}
