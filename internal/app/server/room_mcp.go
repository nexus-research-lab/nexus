package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	roommcp "github.com/nexus-research-lab/nexus/internal/mcp/room"
	roommcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// newRoomMCPBuilder 返回 Room runtime 内建通讯 MCPServerBuilder。
func newRoomMCPBuilder(
	svc roommcpcontract.Service,
	getRoom func(context.Context, string) (*protocol.RoomAggregate, error),
) func(context.Context, *protocol.Agent, string, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
	) map[string]sdkmcp.ServerConfig {
		if svc == nil || strings.TrimSpace(sourceContextType) != "room" {
			return nil
		}
		parsed := protocol.ParseSessionKey(sessionKey)
		if parsed.Kind != protocol.SessionKeyKindRoom || strings.TrimSpace(parsed.ConversationID) == "" {
			return nil
		}
		if agentValue == nil {
			return nil
		}
		sctx := roommcpcontract.ServerContext{
			OwnerUserID:        strings.TrimSpace(agentValue.OwnerUserID),
			CurrentAgentID:     strings.TrimSpace(agentValue.AgentID),
			CurrentSessionKey:  strings.TrimSpace(sessionKey),
			CurrentRoundID:     strings.TrimSpace(roundID),
			RoomID:             strings.TrimSpace(sourceContextID),
			ConversationID:     strings.TrimSpace(parsed.ConversationID),
			SourceContextType:  strings.TrimSpace(sourceContextType),
			SourceContextLabel: strings.TrimSpace(sourceContextLabel),
		}
		if getRoom != nil && strings.TrimSpace(sctx.RoomID) != "" {
			if record, err := getRoom(ctx, sctx.RoomID); err == nil && record != nil {
				sctx.PrivateMessagesEnabled = record.Room.PrivateMessagesEnabled
			}
		}
		return map[string]sdkmcp.ServerConfig{
			roommcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     roommcpcontract.ServerName,
				Instance: roommcp.NewServer(svc, sctx),
			},
		}
	}
}
