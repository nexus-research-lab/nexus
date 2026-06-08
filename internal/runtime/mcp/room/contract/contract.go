// Package contract 定义 nexus_room MCP 子包之间共享的契约。
package contract

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ServerName 是 Room 内建 MCP server 的注册名。
const ServerName = "nexus_room"

// ServerContext 承载当前 Room 成员运行时上下文。
type ServerContext struct {
	OwnerUserID        string
	CurrentAgentID     string
	CurrentSessionKey  string
	RoomID             string
	ConversationID     string
	SourceContextType  string
	SourceContextLabel string
}

// Service 是 nexus_room MCP server 依赖的 Room runtime 子集。
type Service interface {
	HandleDirectedMessage(
		ctx context.Context,
		roomID string,
		conversationID string,
		request protocol.CreateRoomDirectedMessageRequest,
	) (*protocol.RoomDirectedMessageRecord, error)
	HandlePublicMessage(
		ctx context.Context,
		roomID string,
		conversationID string,
		request protocol.CreateRoomPublicMessageRequest,
	) (protocol.Message, error)
}
