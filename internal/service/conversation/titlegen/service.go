package titlegen

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
)

const titleRequestTimeout = 45 * time.Second

// Service 负责按首条用户消息异步生成会话标题。
type Service struct {
	providers providerResolver
	prefs     preferencesService
	sessions  sessionService
	rooms     roomService
	events    eventBroadcaster
	logger    *slog.Logger
	llmClient *llm.Client

	runAsync func(func())

	mu       sync.Mutex
	inflight map[string]struct{}
}

// NewService 创建标题生成服务。
func NewService(
	providers providerResolver,
	sessions sessionService,
	rooms roomService,
	events eventBroadcaster,
	prefs ...preferencesService,
) *Service {
	var preferenceService preferencesService
	if len(prefs) > 0 {
		preferenceService = prefs[0]
	}
	return &Service{
		providers: providers,
		prefs:     preferenceService,
		sessions:  sessions,
		rooms:     rooms,
		events:    events,
		logger:    logx.NewDiscardLogger(),
		llmClient: llm.NewClient(&http.Client{
			Timeout: titleRequestTimeout,
		}),
		runAsync: func(job func()) { go job() },
		inflight: make(map[string]struct{}),
	}
}

// SetLogger 注入日志实例。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// Schedule 异步调度一次标题生成。
func (s *Service) Schedule(ctx context.Context, request Request) {
	if s == nil || s.providers == nil || s.llmClient == nil {
		return
	}
	if strings.TrimSpace(request.Content) == "" {
		s.logger.Debug("跳过标题生成：内容为空",
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
		)
		return
	}
	targetKey := request.targetKey()
	if targetKey == "" {
		s.logger.Debug("跳过标题生成：缺少目标",
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
		)
		return
	}
	if !request.shouldCheckSessionTitle() && !request.shouldCheckConversationTitle() {
		s.logger.Debug("跳过标题生成：标题无需更新",
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
			"session_title", request.SessionTitle,
			"session_message_count", request.SessionMessageCount,
			"conversation_message_count", request.ConversationMessageCount,
		)
		return
	}
	if !s.markInflight(targetKey) {
		s.logger.Debug("跳过标题生成：已有任务执行中",
			"target_key", targetKey,
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
		)
		return
	}

	s.logger.Debug("调度标题生成",
		"target_key", targetKey,
		"session_key", request.SessionKey,
		"conversation_id", request.ConversationID,
		"owner_user_id", request.OwnerUserID,
		"provider", request.Provider,
		"model", request.Model,
		"session_title", request.SessionTitle,
		"session_message_count", request.SessionMessageCount,
	)
	asyncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), titleRequestTimeout)
	s.runAsync(func() {
		defer cancel()
		defer s.clearInflight(targetKey)
		s.generateAndApply(asyncCtx, request)
	})
}

func (s *Service) markInflight(targetKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.inflight[targetKey]; exists {
		return false
	}
	s.inflight[targetKey] = struct{}{}
	return true
}

func (s *Service) clearInflight(targetKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inflight, targetKey)
}
