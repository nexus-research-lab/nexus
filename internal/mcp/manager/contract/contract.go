// INPUT: server 固化的 owner、Agent、session、round 与 DM/Room 上下文。
// OUTPUT: 不可由工具参数覆盖的 nexusmanager.Actor 和 transport 最小服务契约。
// POS: nexus_manager 的可信 transport 边界。
package contract

import (
	"context"

	managersvc "github.com/nexus-research-lab/nexus/internal/service/nexusmanager"
)

// ServerName 是受控 Nexus 资源管理 MCP server 的注册名。
const ServerName = "nexus_manager"

// ServerContext 全部来自 server/runtime 记录，不能由模型参数覆盖。
type ServerContext struct {
	OwnerUserID       string
	CurrentAgentID    string
	CurrentSessionKey string
	CurrentRoundID    string
	LeaseSessionKey   string
	LeaseRoundID      string
	ContextKind       string
	ContextID         string
	RoomID            string
	ConversationID    string
	IsMainAgent       bool
}

// Actor 把服务端上下文转换为每次调用都要重校验的业务身份。
func (s ServerContext) Actor() managersvc.Actor {
	return managersvc.Actor{
		OwnerUserID: s.OwnerUserID, AgentID: s.CurrentAgentID,
		SessionKey: s.CurrentSessionKey, RoundID: s.CurrentRoundID,
		LeaseSessionKey: s.LeaseSessionKey, LeaseRoundID: s.LeaseRoundID,
		ContextKind: s.ContextKind, ContextID: s.ContextID,
		RoomID: s.RoomID, ConversationID: s.ConversationID,
	}
}

// Service 是 nexus_manager MCP 唯一可调用的业务面。
type Service interface {
	InspectCapabilities(context.Context, managersvc.Actor) (*managersvc.CapabilitySnapshot, error)
	ListAgents(context.Context, managersvc.Actor) ([]managersvc.AgentView, error)
	GetAgent(context.Context, managersvc.Actor, string) (*managersvc.AgentView, error)
	ListRooms(context.Context, managersvc.Actor, int) ([]managersvc.RoomView, error)
	GetRoom(context.Context, managersvc.Actor, string) (*managersvc.RoomView, error)
	ListRoomContexts(context.Context, managersvc.Actor, string, int) ([]managersvc.ConversationContextView, error)
	GetConversation(context.Context, managersvc.Actor, string) (*managersvc.ConversationContextView, error)
	ListSessions(context.Context, managersvc.Actor, string, int) ([]managersvc.SessionView, error)
	GetSession(context.Context, managersvc.Actor, string) (*managersvc.SessionView, error)
	ListWorkspaceFiles(context.Context, managersvc.Actor, string, int) (*managersvc.WorkspaceListing, error)
	ReadWorkspaceFile(context.Context, managersvc.Actor, string, string, int) (*managersvc.WorkspaceFileView, error)
}
