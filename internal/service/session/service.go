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

type externalSessionIdentityResolver interface {
	ResolveExternalSessionIdentity(context.Context, string, string) (*protocol.ExternalSessionIdentity, error)
}

type sessionTaskReferenceResolver interface {
	CountTasksReferencingSessions(context.Context, string, []string) (map[string]int, error)
}

type runtimeSettingsPreparationScheduler interface {
	ScheduleRuntimeSettingsPreparation(context.Context, protocol.Session)
}

// SQLRepository 定义 Room Session 视图所需的 SQL 读取能力。
type SQLRepository interface {
	ListRoomSessions(context.Context, string) ([]protocol.Session, error)
	ListRoomSessionsByAgent(context.Context, string) ([]protocol.Session, error)
	GetRoomSessionByKey(context.Context, string, protocol.SessionKey) (*protocol.Session, error)
	UpdateRoomSessionRuntimeIdentity(context.Context, string, string, string) error
	UpdateRoomConversationRuntimeSettings(
		context.Context,
		string,
		map[string]any,
		string,
	) ([]protocol.Session, error)
}

var (
	// ErrSessionNotFound 表示 session 不存在。
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionMutationUnsupported 表示该 session 只能通过更高层语义操作。
	ErrSessionMutationUnsupported = errors.New("session mutation is not supported")
	// ErrSessionConfigurationVersionConflict 表示 session meta 已被其他 writer 推进。
	ErrSessionConfigurationVersionConflict = errors.New("session configuration version conflict")
	// ErrSessionDeleted 表示 session 已进入持久删除栅栏，普通 writer 不得复活。
	ErrSessionDeleted = errors.New("session is deleting or deleted")
	// ErrExternalSessionPairingActive 表示外部 IM 会话仍由有效配对占用。
	ErrExternalSessionPairingActive = errors.New("external IM session pairing is active")
	// ErrMessageDetailUnavailable 表示大内容引用已过期或不属于当前 Session generation。
	ErrMessageDetailUnavailable = workspacestore.ErrHistoryMessageDetailUnavailable
)

// Service 负责编排文件会话与 Room SQL 会话视图。
type Service struct {
	config                     config.Config
	agentService               *agentsvc.Service
	repository                 SQLRepository
	files                      *workspacestore.SessionFileStore
	history                    *workspacestore.AgentHistoryStore
	roomHistory                *workspacestore.RoomHistoryStore
	runtime                    *runtimectx.Manager
	deletion                   *deletionsvc.Coordinator
	notifier                   DirectoryNotifier
	externalIdentity           externalSessionIdentityResolver
	taskReferences             sessionTaskReferenceResolver
	goalUsage                  GoalCompletionUsageProvider
	runtimeSettingsPreparation runtimeSettingsPreparationScheduler

	recoveryMu      sync.Mutex
	recoveryBlocked map[string]struct{}
}

// SetRuntimeSettingsPreparationScheduler 注入 Session 配置提交后的异步 runtime 预备器。
// 持久化成功是控制面提交点；预备失败不得回滚用户设置，下一轮仍由 runtime 主链兜底。
func (s *Service) SetRuntimeSettingsPreparationScheduler(
	scheduler runtimeSettingsPreparationScheduler,
) {
	s.runtimeSettingsPreparation = scheduler
}

// SetExternalSessionIdentityResolver 注入 IM pairing/account 的实时身份投影。
func (s *Service) SetExternalSessionIdentityResolver(resolver externalSessionIdentityResolver) {
	s.externalIdentity = resolver
}

// SetTaskReferenceResolver 注入定时任务引用计数，用于会话身份与删除影响展示。
func (s *Service) SetTaskReferenceResolver(resolver sessionTaskReferenceResolver) {
	s.taskReferences = resolver
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
