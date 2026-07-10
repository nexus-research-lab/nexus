package clientopts

// 本文件验证 Nexus 托管 nxs 时锁定 provider 路由和 AutoDream 唤醒模式。

import (
	"context"
	"testing"
)

func TestBuildAgentClientOptionsMarksNXSAsHostManaged(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:   "nxs",
		WorkspacePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions() error = %v", err)
	}
	if options.Env[nexusAutoDreamWakeModeEnvName] != "host" || options.Env[nexusProviderManagedByHostEnvName] != "1" {
		t.Fatalf("managed env = %#v, want host-managed nxs", options.Env)
	}
}

func TestBuildAgentClientOptionsDoesNotMarkClaudeAsHostManagedNXS(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:   "claude",
		WorkspacePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions() error = %v", err)
	}
	if options.Env[nexusAutoDreamWakeModeEnvName] != "" || options.Env[nexusProviderManagedByHostEnvName] != "" {
		t.Fatalf("managed env = %#v, want Claude unchanged", options.Env)
	}
}
