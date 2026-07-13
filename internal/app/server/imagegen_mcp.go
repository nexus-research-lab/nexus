package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	imagegenmcp "github.com/nexus-research-lab/nexus/internal/mcp/imagegen"
	imagegenmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type imagegenAgentResolver interface {
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

// newImagegenMCPBuilder 返回 DM/Room 实时链路所需的图片生成 MCPServerBuilder。
func newImagegenMCPBuilder(
	svc imagegenmcpcontract.Service,
	agents imagegenAgentResolver,
) func(string, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		agentID string,
		sessionKey string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || agents == nil || strings.TrimSpace(agentID) == "" {
			return nil
		}
		record, err := agents.GetAgent(context.Background(), agentID)
		if err != nil || record == nil || strings.TrimSpace(record.WorkspacePath) == "" {
			return nil
		}
		sctx := imagegenmcpcontract.ServerContext{
			OwnerUserID:   strings.TrimSpace(record.OwnerUserID),
			WorkspacePath: strings.TrimSpace(record.WorkspacePath),
		}
		return map[string]sdkmcp.ServerConfig{
			imagegenmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     imagegenmcpcontract.ServerName,
				Instance: imagegenmcp.NewServer(svc, sctx),
			},
		}
	}
}
