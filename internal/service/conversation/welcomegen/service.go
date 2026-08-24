// INPUT: 新建 conversation 聚合与应用生命周期信号。
// OUTPUT: 每个 conversation 至多一个后台欢迎语任务。
// POS: 欢迎语异步调度与关闭边界。
package welcomegen

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const welcomeRequestTimeout = 15 * time.Second

// Service 生成并持久化首次 DM/Room 欢迎语。
type Service struct {
	providers providerResolver
	prefs     preferencesService
	agents    agentResolver
	history   *workspacestore.AgentHistoryStore
	rooms     *workspacestore.RoomHistoryStore
	events    roomResyncBroadcaster
	logger    *slog.Logger

	generateText func(context.Context, llm.GenerateTextRequest) (string, error)
	runAsync     func(func())

	mu       sync.Mutex
	inflight map[string]struct{}

	lifecycleMu     sync.Mutex
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	closed          bool
	background      sync.WaitGroup
}

// NewService 创建欢迎语生成服务。
func NewService(
	cfg config.Config,
	providers providerResolver,
	prefs preferencesService,
	agents agentResolver,
) *Service {
	client := llm.NewClient(&http.Client{Timeout: welcomeRequestTimeout})
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	return &Service{
		providers:       providers,
		prefs:           prefs,
		agents:          agents,
		history:         workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		rooms:           workspacestore.NewRoomHistoryStore(cfg.WorkspacePath),
		logger:          logx.NewDiscardLogger(),
		generateText:    client.GenerateText,
		runAsync:        func(job func()) { go job() },
		inflight:        make(map[string]struct{}),
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
	}
}

// SetLogger 注入日志实例。
func (s *Service) SetLogger(logger *slog.Logger) {
	if s == nil {
		return
	}
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetRoomResyncBroadcaster 注入欢迎语写入后的 Room 历史失效投影。
func (s *Service) SetRoomResyncBroadcaster(events roomResyncBroadcaster) {
	if s != nil {
		s.events = events
	}
}

// Schedule 为刚创建的主 conversation 异步生成一次欢迎语。
func (s *Service) Schedule(ctx context.Context, aggregate protocol.ConversationContextAggregate) {
	if s == nil || strings.TrimSpace(aggregate.Conversation.ID) == "" || aggregate.Room.IsContactChannel {
		return
	}
	conversationID := strings.TrimSpace(aggregate.Conversation.ID)
	if !s.markInflight(conversationID) {
		return
	}

	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		s.clearInflight(conversationID)
		return
	}
	s.background.Add(1)
	lifecycleCtx := s.lifecycleCtx
	s.lifecycleMu.Unlock()

	asyncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), welcomeRequestTimeout)
	stopOnClose := context.AfterFunc(lifecycleCtx, cancel)
	s.runAsync(func() {
		defer s.background.Done()
		defer stopOnClose()
		defer cancel()
		defer s.clearInflight(conversationID)
		s.generateAndPersist(asyncCtx, aggregate)
	})
}

// Close 取消并等待仍可能写入 owner 历史的欢迎语任务。
func (s *Service) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.lifecycleMu.Lock()
	if !s.closed {
		s.closed = true
		s.lifecycleCancel()
	}
	s.lifecycleMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.background.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) markInflight(conversationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.inflight[conversationID]; exists {
		return false
	}
	s.inflight[conversationID] = struct{}{}
	return true
}

func (s *Service) clearInflight(conversationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inflight, conversationID)
}
