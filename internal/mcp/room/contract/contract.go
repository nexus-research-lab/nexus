// Package contract 定义 nexus_room MCP 子包之间共享的契约。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package contract

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ServerName 是 Room 内建 MCP server 的注册名。
const ServerName = "nexus_room"

// IsRoomRuntimeSourceContextType 判断来源是否仍属于 Room runtime。
// room_* 只降低配置写权限，不应切断当前 Room 成员的通讯能力。
func IsRoomRuntimeSourceContextType(value string) bool {
	normalized := strings.TrimSpace(value)
	return normalized == "room" || strings.HasPrefix(normalized, "room_")
}

// ServerContext 承载当前 Room 成员运行时上下文。
type ServerContext struct {
	OwnerUserID            string
	CurrentAgentID         string
	CurrentSessionKey      string
	CurrentRoundID         string
	RoomID                 string
	ConversationID         string
	SourceContextType      string
	SourceContextLabel     string
	PrivateMessagesEnabled bool
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
	MarkPublicMessagePublished(
		ctx context.Context,
		sessionKey string,
		roundID string,
		agentID string,
	) error
}
