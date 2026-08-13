// INPUT: Room service 的配置、依赖、owner-scoped repository 与 Session artifact 删除协调器。
// OUTPUT: Room 查询、授权快照、配置/成员/对话 mutation 与统一 tombstone 清理服务。
// POS: Room 持久化业务边界；实时执行只依赖本服务，不反向装配 realtime。
package room

import (
	"context"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	// ErrRoomNotFound 表示房间不存在。
	ErrRoomNotFound = errors.New("room not found")
	// ErrConversationNotFound 表示房间对话不存在。
	ErrConversationNotFound = errors.New("conversation not found")
	// ErrRoomMemberNotFound 表示房间成员不存在。
	ErrRoomMemberNotFound = errors.New("room member not found")
	// ErrSessionArtifactDeletionCoordinatorUnavailable 表示 Room artifact 删除缺少统一协调器。
	ErrSessionArtifactDeletionCoordinatorUnavailable = errors.New(
		"Room Session artifact 删除协调器未装配",
	)
)

// Repository 定义 Room 存储接口。
type Repository interface {
	LoadAgentRuntimeRefs(context.Context, string, []string) ([]roomrepo.AgentRuntimeRef, error)
	ListRecentRooms(context.Context, string, int) ([]protocol.RoomAggregate, error)
	GetRoom(context.Context, string, string) (*protocol.RoomAggregate, error)
	GetRoomAuthorizationSnapshot(context.Context, string, string, string) (*protocol.RoomAuthorizationSnapshot, error)
	GetRoomContexts(context.Context, string, string) ([]protocol.ConversationContextAggregate, error)
	GetConversationContext(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	GetConversationContextForSystem(context.Context, string) (*protocol.ConversationContextAggregate, error)
	FindDMRoomContext(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	FindContactRoomContext(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
	CreateRoom(context.Context, roomrepo.CreateRoomBundle) (*protocol.ConversationContextAggregate, error)
	UpdateRoom(context.Context, string, string, roomrepo.UpdateRoomPatch) (*protocol.ConversationContextAggregate, error)
	AddRoomMember(context.Context, string, string, roomrepo.AgentRuntimeRef) (*protocol.ConversationContextAggregate, error)
	RemoveRoomMember(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
	SetRoomMemberParticipation(context.Context, string, string, string, bool) (*protocol.ConversationContextAggregate, error)
	DeleteRoom(context.Context, string, string) (bool, error)
	DeleteRoomAtVersion(context.Context, string, string, int64) (bool, error)
	CreateConversation(context.Context, roomrepo.CreateConversationBundle) (*protocol.ConversationContextAggregate, error)
	UpdateConversation(context.Context, string, string, string, string) (*protocol.ConversationContextAggregate, error)
	UpdateConversationAtVersion(context.Context, string, string, string, string, int64) (*protocol.ConversationContextAggregate, error)
	DeleteConversation(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
	DeleteConversationAtVersion(context.Context, string, string, string, int64) (*protocol.ConversationContextAggregate, error)
	SetRoomDraftConversation(context.Context, string, string, string) error
	HasConversationReferences(context.Context, string, string, string, []string) (bool, error)
	UpdateSessionSDKSessionID(context.Context, string, string) error
	TouchConversationActivity(context.Context, string, time.Time) error
	MarkConversationStarted(context.Context, string, time.Time) error
}

type goalCleaner interface {
	DeleteGoalsForRoomConversations(context.Context, []string) (int, error)
	DeleteGoalsForRoomMember(context.Context, string, []string) (int, error)
}

type goalConversationInspector interface {
	HasGoalForRoomConversation(context.Context, string) (bool, error)
}

type runtimeSessionCloser interface {
	CloseSession(context.Context, string) error
}

// SessionArtifactDeletionCoordinator 统一撤销 Room 成员 Session 的 runtime 与持久 artifact。
type SessionArtifactDeletionCoordinator interface {
	DeleteSessionArtifacts(context.Context, string, string, string, string) error
}

// Service 提供 Room 编排能力。
type Service struct {
	config           config.Config
	agents           *agentsvc.Service
	repository       Repository
	files            *workspacestore.SessionFileStore
	history          *workspacestore.AgentHistoryStore
	roomHistory      *workspacestore.RoomHistoryStore
	skills           RoomSkillCatalog
	goals            goalCleaner
	goalReader       goalConversationInspector
	runtime          runtimeSessionCloser
	sessionArtifacts SessionArtifactDeletionCoordinator
	deletion         *deletionsvc.Coordinator
}

// NewService 创建 Room 服务。
func NewService(cfg config.Config, agents *agentsvc.Service, repository Repository) *Service {
	return &Service{
		config:      cfg,
		agents:      agents,
		repository:  repository,
		files:       workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:     workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		roomHistory: workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
	}
}

// SetGoalCleaner 注入 Room 删除时的 Goal 级联清理器。
func (s *Service) SetGoalCleaner(cleaner goalCleaner) {
	s.goals = cleaner
	s.goalReader, _ = cleaner.(goalConversationInspector)
}

// SetRuntimeManager 注入运行时管理器，用于关闭 Room conversation 对应的后台 client。
func (s *Service) SetRuntimeManager(runtimeManager runtimeSessionCloser) {
	s.runtime = runtimeManager
}

// SetSessionArtifactDeletionCoordinator 注入 Room 成员 Session 的统一删除协调器。
func (s *Service) SetSessionArtifactDeletionCoordinator(
	coordinator SessionArtifactDeletionCoordinator,
) {
	s.sessionArtifacts = coordinator
}

// SetDeletionCoordinator 注入跨数据库与文件系统的持久删除协调器。
func (s *Service) SetDeletionCoordinator(coordinator *deletionsvc.Coordinator) {
	s.deletion = coordinator
}
