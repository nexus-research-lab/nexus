package server

import (
	"context"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
)

type stubImagegenAgentResolver struct {
	record *protocol.Agent
	err    error
}

func (s stubImagegenAgentResolver) GetAgent(context.Context, string) (*protocol.Agent, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.record, nil
}

type stubImagegenMCPService struct{}

func (stubImagegenMCPService) GenerateImage(
	context.Context,
	imagegensvc.GenerateInput,
) (*imagegensvc.Result, []byte, error) {
	return &imagegensvc.Result{}, nil, nil
}

func (stubImagegenMCPService) EditImage(
	context.Context,
	imagegensvc.EditInput,
) (*imagegensvc.Result, []byte, error) {
	return &imagegensvc.Result{}, nil, nil
}

func TestImagegenMCPBuilderAddsSDKServerForAgentRuntime(t *testing.T) {
	builder := newImagegenMCPBuilder(stubImagegenMCPService{}, stubImagegenAgentResolver{
		record: &protocol.Agent{
			AgentID:       "agent-1",
			Name:          "Painter",
			WorkspacePath: "/workspace/agent-1",
			OwnerUserID:   "user-1",
		},
	})

	servers := builder("agent-1", "agent:agent-1:ws:dm:session-1", "agent", "agent-1", "Painter")
	config, ok := servers["nexus_imagegen"].(sdkmcp.SDKServerConfig)
	if !ok {
		t.Fatalf("Agent runtime 应注入 nexus_imagegen SDK server: %+v", servers)
	}
	if config.Name != "nexus_imagegen" || config.Instance == nil {
		t.Fatalf("nexus_imagegen SDK server 配置不正确: %+v", config)
	}
}

func TestImagegenMCPBuilderSkipsMissingWorkspace(t *testing.T) {
	builder := newImagegenMCPBuilder(stubImagegenMCPService{}, stubImagegenAgentResolver{
		record: &protocol.Agent{AgentID: "agent-1"},
	})

	if servers := builder("agent-1", "session", "agent", "agent-1", "Agent"); len(servers) != 0 {
		t.Fatalf("缺少 workspace 时不应注入 nexus_imagegen: %+v", servers)
	}
}
