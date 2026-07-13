package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	roommcp "github.com/nexus-research-lab/nexus/internal/mcp/room"
	roommcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/agent"
)

// newRoomMCPBuilder 返回 Room runtime 内建通讯 MCPServerBuilder。
func newRoomMCPBuilder(
	svc roommcpcontract.Service,
	agents *agent.Service,
	getRoom func(context.Context, string) (*protocol.RoomAggregate, error),
) func(string, string, string, string, string) map[string]sdkmcp.ServerConfig {
	return func(
		agentID string,
		sessionKey string,
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
		sctx := roommcpcontract.ServerContext{
			CurrentAgentID:     strings.TrimSpace(agentID),
			CurrentSessionKey:  strings.TrimSpace(sessionKey),
			RoomID:             strings.TrimSpace(sourceContextID),
			ConversationID:     strings.TrimSpace(parsed.ConversationID),
			SourceContextType:  strings.TrimSpace(sourceContextType),
			SourceContextLabel: strings.TrimSpace(sourceContextLabel),
		}
		if agents != nil && strings.TrimSpace(agentID) != "" {
			if record, err := agents.GetAgent(context.Background(), agentID); err == nil && record != nil {
				sctx.OwnerUserID = strings.TrimSpace(record.OwnerUserID)
			}
		}
		if getRoom != nil && strings.TrimSpace(sctx.RoomID) != "" {
			roomCtx := context.Background()
			if sctx.OwnerUserID != "" {
				roomCtx = authctx.WithPrincipal(roomCtx, &authctx.Principal{UserID: sctx.OwnerUserID})
			}
			if record, err := getRoom(roomCtx, sctx.RoomID); err == nil && record != nil {
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
