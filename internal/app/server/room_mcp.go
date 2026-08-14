// INPUT: Room runtime 身份、Room 当前配置与 realtime 通讯服务。
// OUTPUT: 仅普通 Group Room 可见、工具面稳定且按私域开关鉴权的 nexus_room MCP builder。
// POS: Room 内协作工具装配边界；联系人内部通道只使用 nexus_comms/reply route。
package server

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	roommcp "github.com/nexus-research-lab/nexus/internal/mcp/room"
	roommcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
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
		if svc == nil || !roommcpcontract.IsRoomRuntimeSourceContextType(sourceContextType) {
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
		if lease, ok := runtimectx.MCPRoundLeaseFromContext(ctx); ok {
			sctx.CurrentAgentRoundID = strings.TrimSpace(lease.RoundID)
		}
		if getRoom != nil && strings.TrimSpace(sctx.RoomID) != "" {
			if record, err := getRoom(ctx, sctx.RoomID); err == nil && record != nil {
				if record.Room.IsContactChannel {
					return nil
				}
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
