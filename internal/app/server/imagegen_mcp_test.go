package server

import (
	"context"
	"errors"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

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

type stubImagegenMCPConfigResolver struct {
	err error
}

func (s stubImagegenMCPConfigResolver) ResolveImageConfig(
	context.Context,
	string,
) (*providercfg.ImageConfig, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &providercfg.ImageConfig{Provider: "image-provider", Model: "image-model"}, nil
}

func TestImagegenMCPBuilderAddsSDKServerForAgentRuntime(t *testing.T) {
	builder := newImagegenMCPBuilder(
		stubImagegenMCPService{},
		stubImagegenMCPConfigResolver{},
	)
	agentValue := &protocol.Agent{
		AgentID:       "agent-1",
		Name:          "Painter",
		WorkspacePath: "/workspace/agent-1",
		OwnerUserID:   "user-1",
	}

	servers := builder(
		context.Background(),
		agentValue,
		"agent:agent-1:ws:dm:session-1",
		"agent",
		"agent-1",
		"Painter",
	)
	config, ok := servers["nexus_imagegen"].(sdkmcp.SDKServerConfig)
	if !ok {
		t.Fatalf("Agent runtime 应注入 nexus_imagegen SDK server: %+v", servers)
	}
	if config.Name != "nexus_imagegen" || config.Instance == nil {
		t.Fatalf("nexus_imagegen SDK server 配置不正确: %+v", config)
	}
}

func TestImagegenMCPBuilderSkipsMissingWorkspace(t *testing.T) {
	builder := newImagegenMCPBuilder(
		stubImagegenMCPService{},
		stubImagegenMCPConfigResolver{},
	)

	if servers := builder(
		context.Background(),
		&protocol.Agent{AgentID: "agent-1"},
		"session",
		"agent",
		"agent-1",
		"Agent",
	); len(servers) != 0 {
		t.Fatalf("缺少 workspace 时不应注入 nexus_imagegen: %+v", servers)
	}
}

func TestImagegenMCPBuilderSkipsMissingImageModel(t *testing.T) {
	builder := newImagegenMCPBuilder(
		stubImagegenMCPService{},
		stubImagegenMCPConfigResolver{err: errors.New("未配置生图模型")},
	)

	if servers := builder(
		context.Background(),
		&protocol.Agent{
			AgentID:       "agent-1",
			WorkspacePath: "/workspace/agent-1",
		},
		"session",
		"agent",
		"agent-1",
		"Agent",
	); len(servers) != 0 {
		t.Fatalf("缺少生图模型时不应注入 nexus_imagegen: %+v", servers)
	}
}
