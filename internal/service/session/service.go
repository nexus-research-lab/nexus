package session

import (
	"errors"

	"github.com/nexus-research-lab/nexus/internal/config"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	// ErrSessionNotFound 表示 session 不存在。
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionMutationUnsupported 表示该 session 只能通过更高层语义操作。
	ErrSessionMutationUnsupported = errors.New("session mutation is not supported")
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
	notifier     DirectoryNotifier
}

// SetRuntimeManager 注入运行时管理器，用于历史读取时识别活跃轮次。
func (s *Service) SetRuntimeManager(runtimeManager *runtimectx.Manager) {
	s.runtime = runtimeManager
}

// SetDirectoryNotifier 注入目录变更通知器。
func (s *Service) SetDirectoryNotifier(notifier DirectoryNotifier) {
	s.notifier = notifier
}

// NewService 使用已注入的依赖创建 Session 服务。
func NewService(cfg config.Config, agentService *agentsvc.Service, repository SQLRepository) *Service {
	return &Service{
		config:       cfg,
		agentService: agentService,
		repository:   repository,
		files:        workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		history:      workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		roomHistory:  workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
	}
}
