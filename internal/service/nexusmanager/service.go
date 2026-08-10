// INPUT: Agent、Room、Session、workspace 与 runtime round 的消费侧窄接口。
// OUTPUT: 不依赖 handler/app 的 nexus_manager 服务。
// POS: nexus_manager 依赖根，只装配明确允许的资源域。
package nexusmanager

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

type agentService interface {
	ListAgents(context.Context) ([]protocol.Agent, error)
	GetAgent(context.Context, string) (*protocol.Agent, error)
}

type roomService interface {
	ListRooms(context.Context, int) ([]protocol.RoomAggregate, error)
	GetRoom(context.Context, string) (*protocol.RoomAggregate, error)
	GetRoomAuthorizationSnapshot(context.Context, string, string) (*protocol.RoomAuthorizationSnapshot, error)
	GetRoomContexts(context.Context, string) ([]protocol.ConversationContextAggregate, error)
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
}

type sessionService interface {
	ListSessions(context.Context) ([]protocol.Session, error)
	ListAgentSessions(context.Context, string) ([]protocol.Session, error)
	GetSession(context.Context, string) (*protocol.Session, error)
}

type workspaceService interface {
	ListFiles(context.Context, string) ([]workspacepkg.FileEntry, error)
	GetFile(context.Context, string, string) (*workspacepkg.FileContent, error)
}

type roundLeaseVerifier interface {
	GetRunningRoundIDs(string) []string
}

// Service 只组合资源查询和 workspace 读取。
type Service struct {
	agents     agentService
	rooms      roomService
	sessions   sessionService
	workspaces workspaceService
	runtime    roundLeaseVerifier
}

// NewService 创建受控 Nexus 管理服务。
func NewService(
	agents agentService,
	rooms roomService,
	sessions sessionService,
	workspaces workspaceService,
	runtime roundLeaseVerifier,
) *Service {
	return &Service{
		agents: agents, rooms: rooms, sessions: sessions,
		workspaces: workspaces, runtime: runtime,
	}
}

func (s *Service) requireRooms() error {
	if s == nil || s.rooms == nil {
		return errors.New("nexus_manager 未装配 Room 服务")
	}
	return nil
}

func (s *Service) requireSessions() error {
	if s == nil || s.sessions == nil {
		return errors.New("nexus_manager 未装配 Session 服务")
	}
	return nil
}

func (s *Service) requireWorkspaces() error {
	if s == nil || s.workspaces == nil {
		return errors.New("nexus_manager 未装配 Workspace 服务")
	}
	return nil
}
