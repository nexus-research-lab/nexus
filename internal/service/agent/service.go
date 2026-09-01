// INPUT: Agent 仓储、主机配置及可选 Goal/删除协调依赖。
// OUTPUT: 可装配的 Agent 业务服务。
// POS: Agent 服务依赖根，只声明消费侧窄接口而不反向依赖其他业务域。
package agent

import (
	"context"
	"errors"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type goalCleaner interface {
	DeleteGoalsForAgent(context.Context, string) (int, error)
}

type agentSessionLifecycle interface {
	ListAgentSessions(context.Context, string) ([]protocol.Session, error)
	DeleteAgentSessionArtifacts(context.Context, protocol.Agent, []protocol.Session) error
}

type agentTaskCleaner interface {
	DeleteTasksForAgent(context.Context, string, string) error
}

// workspaceManager 管理显式 Agent 生命周期产生的 workspace 托管状态，不参与全局 readiness。
type workspaceManager interface {
	InitializeAgentWorkspace(context.Context, protocol.Agent) error
	RemoveAgentWorkspaceState(context.Context, protocol.Agent) error
}

var (
	// ErrAgentNotFound 表示 Agent 不存在。
	ErrAgentNotFound = errors.New("agent not found")
	// ErrAgentNameInvalid 表示 Agent 名称格式不合法。
	ErrAgentNameInvalid = errors.New("agent name invalid")
	// ErrMainAgentNameImmutable 表示主智能体名称校验在写入前被拒绝。
	ErrMainAgentNameImmutable = errors.New("main agent name is immutable")
	// ErrAgentDeletionNotAllowed 表示目标 Agent 属于宿主控制面，删除没有进入协调阶段。
	ErrAgentDeletionNotAllowed = errors.New("agent deletion not allowed")
	// ErrRuntimeVersionConflict 表示 Agent 已被其他写入更新。
	ErrRuntimeVersionConflict = agentrepo.ErrRuntimeVersionConflict
)

// Service 提供 Agent 业务能力。
type Service struct {
	config              config.Config
	repository          Repository
	history             *workspacestore.AgentHistoryStore
	prompts             *promptBuilder
	goals               goalCleaner
	workspace           workspaceManager
	sessions            agentSessionLifecycle
	tasks               agentTaskCleaner
	deletionCoordinator deletionCoordinator
	readyMu             sync.Mutex
	creationLocksMu     sync.Mutex
	creationLocks       map[string]*agentCreationLock
}

// NewService 创建 Agent 服务。
func NewService(cfg config.Config, repository Repository) *Service {
	return &Service{
		config:     cfg,
		repository: repository,
		history:    workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		prompts:    newPromptBuilder(cfg),
	}
}

// SetGoalCleaner 注入 Agent 删除时的 Goal 级联清理器。
func (s *Service) SetGoalCleaner(cleaner goalCleaner) {
	s.goals = cleaner
}

// SetDeletionLifecycle 注入 Agent 删除涉及的 Session 与 Task 清理器。
func (s *Service) SetDeletionLifecycle(
	sessions agentSessionLifecycle,
	tasks agentTaskCleaner,
) {
	s.sessions = sessions
	s.tasks = tasks
}

// SetWorkspaceManager 注入显式 Agent 生命周期的 workspace 托管器。
func (s *Service) SetWorkspaceManager(manager workspaceManager) {
	s.workspace = manager
}

func (s *Service) cleanupAgentWorkspace(ctx context.Context, agentValue protocol.Agent) error {
	workspaceErr := s.removeAgentWorkspace(agentValue)
	if s.workspace != nil {
		// 初始化 marker 只是可重建缓存，清理失败不能把已删除的目录
		// 变成仍留在数据库中的残缺 Agent。
		_ = s.workspace.RemoveAgentWorkspaceState(ctx, agentValue)
	}
	return workspaceErr
}

func (s *Service) initializeAgentWorkspace(ctx context.Context, agentValue protocol.Agent) error {
	if s.workspace == nil {
		return nil
	}
	return s.workspace.InitializeAgentWorkspace(ctx, agentValue)
}

// SetDeletionCoordinator 注入其他能力域在 Agent 持久删除前后的锁定、核验和运行态撤销。
func (s *Service) SetDeletionCoordinator(coordinator deletionCoordinator) {
	s.deletionCoordinator = coordinator
}
