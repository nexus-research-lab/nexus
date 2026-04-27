package automation

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

type dmRunner interface {
	HandleChat(context.Context, dmsvc.Request) error
}

type roomRunner interface {
	HandleChat(context.Context, roomsvc.ChatRequest) error
}

type workspaceReader interface {
	GetFile(context.Context, string, string) (*workspacepkg.FileContent, error)
}

type deliveryRouter interface {
	DeliverText(context.Context, string, string, channels.DeliveryTarget) (channels.DeliveryTarget, error)
}

type runtimeSessionCloser interface {
	CloseSession(context.Context, string) error
}

// Service 提供 scheduled tasks 与 heartbeat 的真实业务能力。
type Service struct {
	config        config.Config
	repository    *automationstore.Repository
	agents        *agentsvc.Service
	dm            dmRunner
	room          roomRunner
	permission    *permissionctx.Context
	workspace     workspaceReader
	delivery      deliveryRouter
	logger        *slog.Logger
	sessionCloser runtimeSessionCloser

	nowFn     func() time.Time
	idFactory func(string) string

	mu             sync.Mutex
	jobStates      map[string]*automationdomain.JobRuntimeState
	heartbeatState map[string]*automationdomain.HeartbeatRuntimeState
	wakeRequests   map[string][]automationdomain.HeartbeatWakeRequest
	started        bool
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewService 创建自动化服务。
func NewService(
	cfg config.Config,
	db *sql.DB,
	agents *agentsvc.Service,
	dm dmRunner,
	room roomRunner,
	permission *permissionctx.Context,
	workspace workspaceReader,
	delivery deliveryRouter,
) *Service {
	return &Service{
		config:         cfg,
		repository:     automationstore.NewRepository(cfg, db),
		agents:         agents,
		dm:             dm,
		room:           room,
		permission:     permission,
		workspace:      workspace,
		delivery:       delivery,
		logger:         logx.NewDiscardLogger(),
		nowFn:          func() time.Time { return time.Now().UTC() },
		idFactory:      automationdomain.NewID,
		jobStates:      make(map[string]*automationdomain.JobRuntimeState),
		heartbeatState: make(map[string]*automationdomain.HeartbeatRuntimeState),
		wakeRequests:   make(map[string][]automationdomain.HeartbeatWakeRequest),
	}
}

// SetLogger 注入业务日志实例。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetRuntimeSessionCloser 注入运行时会话关闭器，用于清理 isolated 自动化会话。
func (s *Service) SetRuntimeSessionCloser(sessionCloser runtimeSessionCloser) {
	s.sessionCloser = sessionCloser
}

// Start 启动后台调度循环。
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	if s.agents != nil {
		if err := s.agents.EnsureReady(ctx); err != nil {
			return err
		}
	}
	if err := s.bootstrapRuntime(ctx); err != nil {
		return err
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	s.wg.Add(1)
	s.loggerFor(ctx).Info("自动化调度器已启动")
	go s.runLoop(loopCtx)
	return nil
}

// Stop 停止后台调度循环。
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.started = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
	s.loggerFor(context.Background()).Info("自动化调度器已停止")
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

func (s *Service) ensureReady(ctx context.Context) error {
	if s.agents == nil {
		return nil
	}
	return s.agents.EnsureReady(ctx)
}

func (s *Service) requireAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	if s.agents == nil {
		return nil, nil
	}
	return s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
}

func (s *Service) validateAgentAndTarget(ctx context.Context, agentID string, target protocol.SessionTarget) error {
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return err
	}
	if strings.TrimSpace(target.Kind) != protocol.SessionTargetBound {
		return nil
	}
	parsed := protocol.ParseSessionKey(target.BoundSessionKey)
	if parsed.Kind == protocol.SessionKeyKindAgent && parsed.AgentID != "" && parsed.AgentID != strings.TrimSpace(agentID) {
		return errors.New("agent_id 与 session_target 不一致")
	}
	return nil
}

func (s *Service) ensureDirectTargetSupported(target protocol.SessionTarget) error {
	if strings.TrimSpace(target.Kind) == protocol.SessionTargetMain {
		return nil
	}
	sessionKey, err := automationdomain.ResolveSessionKey(protocol.CronJob{
		AgentID:       "noop",
		SessionTarget: target,
	}, stringPointer("noop"))
	if err != nil {
		return err
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return errors.New("shared room session automation 暂不支持")
	}
	return nil
}
