package room

import (
	"context"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
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
)

// Repository 定义 Room 存储接口。
type Repository interface {
	LoadAgentRuntimeRefs(context.Context, string, []string) ([]roomrepo.AgentRuntimeRef, error)
	ListRecentRooms(context.Context, string, int) ([]protocol.RoomAggregate, error)
	GetRoom(context.Context, string, string) (*protocol.RoomAggregate, error)
	GetRoomContexts(context.Context, string, string) ([]protocol.ConversationContextAggregate, error)
	GetConversationContext(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	GetConversationContextForSystem(context.Context, string) (*protocol.ConversationContextAggregate, error)
	FindDMRoomContext(context.Context, string, string) (*protocol.ConversationContextAggregate, error)
	CreateRoom(context.Context, roomrepo.CreateRoomBundle) (*protocol.ConversationContextAggregate, error)
	UpdateRoom(context.Context, string, string, roomrepo.UpdateRoomPatch) (*protocol.ConversationContextAggregate, error)
	AddRoomMember(context.Context, string, string, roomrepo.AgentRuntimeRef) (*protocol.ConversationContextAggregate, error)
	RemoveRoomMember(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
	DeleteRoom(context.Context, string, string) (bool, error)
	CreateConversation(context.Context, roomrepo.CreateConversationBundle) (*protocol.ConversationContextAggregate, error)
	UpdateConversation(context.Context, string, string, string, string) (*protocol.ConversationContextAggregate, error)
	DeleteConversation(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
	UpdateSessionSDKSessionID(context.Context, string, string) error
	TouchConversationActivity(context.Context, string, time.Time) error
}

type goalCleaner interface {
	DeleteGoalsForRoomConversations(context.Context, []string) (int, error)
	DeleteGoalsForRoomMember(context.Context, string, []string) (int, error)
}

type runtimeSessionCloser interface {
	CloseSession(context.Context, string) error
}

type quotaChecker interface {
	EnsureQuotaAvailable(context.Context, string) error
}

// Service 提供 Room 编排能力。
type Service struct {
	config     config.Config
	agents     *agentsvc.Service
	repository Repository
	files      *workspacestore.SessionFileStore
	history    *workspacestore.AgentHistoryStore
	skills     RoomSkillCatalog
	goals      goalCleaner
	runtime    runtimeSessionCloser
}

// NewService 创建 Room 服务。
func NewService(cfg config.Config, agents *agentsvc.Service, repository Repository) *Service {
	return &Service{
		config:     cfg,
		agents:     agents,
		repository: repository,
		files:      workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:    workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
	}
}

// SetGoalCleaner 注入 Room 删除时的 Goal 级联清理器。
func (s *Service) SetGoalCleaner(cleaner goalCleaner) {
	s.goals = cleaner
}

// SetRuntimeManager 注入运行时管理器，用于关闭 Room conversation 对应的后台 client。
func (s *Service) SetRuntimeManager(runtimeManager runtimeSessionCloser) {
	s.runtime = runtimeManager
}

// SetQuotaChecker 注入订阅额度检查器。
func (s *RealtimeService) SetQuotaChecker(checker quotaChecker) {
	s.quota = checker
}

func (s *RealtimeService) ensureQuotaAvailable(ctx context.Context) error {
	if s.quota == nil {
		return nil
	}
	return s.quota.EnsureQuotaAvailable(ctx, authctx.OwnerUserID(ctx))
}
