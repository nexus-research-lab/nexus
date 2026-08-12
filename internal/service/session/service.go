// INPUT: Session 存储、workspace history、runtime 与跨域只读依赖。
// OUTPUT: owner-scoped Session 服务及其窄依赖注入契约。
// POS: Session 业务服务装配与生命周期边界。
package session

import (
	"context"
	"errors"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	// ErrSessionNotFound 表示 session 不存在。
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionMutationUnsupported 表示该 session 只能通过更高层语义操作。
	ErrSessionMutationUnsupported = errors.New("session mutation is not supported")
	// ErrSessionConfigurationVersionConflict 表示 session meta 已被其他 writer 推进。
	ErrSessionConfigurationVersionConflict = errors.New("session configuration version conflict")
	// ErrSessionDeleted 表示 session 已进入持久删除栅栏，普通 writer 不得复活。
	ErrSessionDeleted = errors.New("session is deleting or deleted")
)

// Service 负责编排文件会话与 Room SQL 会话视图。
type Service struct {
	config       config.Config
	agentService *agentsvc.Service
	repository   SQLRepository
	files        *workspacestore.SessionFileStore
	history      *workspacestore.AgentHistoryStore
	roomHistory  *workspacestore.RoomHistoryStore
	runtime      *runtimectx.Manager
	deletion     *deletionsvc.Coordinator
	notifier     DirectoryNotifier
	goalUsage    GoalCompletionUsageProvider

	recoveryMu      sync.Mutex
	recoveryBlocked map[string]struct{}
}

// GoalCompletionUsageProvider 提供历史完成收据的当前 Goal 聚合真相。
type GoalCompletionUsageProvider interface {
	UsageByGoalIDForOwner(context.Context, string, string) (*protocol.GoalUsageReport, error)
}

// SetGoalCompletionUsageProvider 注入 Goal 聚合读取器，用于修复历史收据中的旧结算值。
func (s *Service) SetGoalCompletionUsageProvider(provider GoalCompletionUsageProvider) {
	s.goalUsage = provider
}

// SetRuntimeManager 注入运行时管理器，用于历史读取与删除前关闭活跃会话。
func (s *Service) SetRuntimeManager(runtimeManager *runtimectx.Manager) {
	s.runtime = runtimeManager
}

// SetDeletionCoordinator 注入跨数据库与文件系统的持久删除协调器。
func (s *Service) SetDeletionCoordinator(coordinator *deletionsvc.Coordinator) {
	s.deletion = coordinator
}

// SetDirectoryNotifier 注入目录变更通知器。
func (s *Service) SetDirectoryNotifier(notifier DirectoryNotifier) {
	s.notifier = notifier
}

// NewService 使用已注入的依赖创建 Session 服务。
func NewService(cfg config.Config, agentService *agentsvc.Service, repository SQLRepository) *Service {
	return &Service{
		config:          cfg,
		agentService:    agentService,
		repository:      repository,
		files:           workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:         workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		roomHistory:     workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
		recoveryBlocked: make(map[string]struct{}),
	}
}

func (s *Service) ownerFiles(ctx context.Context) *workspacestore.SessionFileStore {
	return s.files.ForOwner(authctx.OwnerUserID(ctx))
}

func (s *Service) ownerHistory(ctx context.Context) *workspacestore.AgentHistoryStore {
	return s.history.ForOwner(authctx.OwnerUserID(ctx))
}
