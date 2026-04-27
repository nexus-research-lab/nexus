package titlegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

const (
	titleRequestTimeout = 45 * time.Second
	titleAttemptTimeout = 20 * time.Second
	titleMaxTokens      = 32
	titleMaxAttempts    = 2
)

var (
	defaultConversationPattern = regexp.MustCompile(`^.+\s·\s对话\s+\d+$`)
	whitespacePattern          = regexp.MustCompile(`\s+`)
)

// Request 描述一次标题生成请求。
type Request struct {
	SessionKey               string
	Provider                 string
	Content                  string
	SessionTitle             string
	SessionMessageCount      int
	ConversationID           string
	ConversationRoomID       string
	ConversationTitle        string
	ConversationRoomName     string
	ConversationMessageCount int
}

type providerResolver interface {
	ResolveRuntimeConfig(context.Context, string) (*providercfg.RuntimeConfig, error)
}

type sessionService interface {
	GetSession(context.Context, string) (*protocol.Session, error)
	UpdateSessionTitle(context.Context, string, string) (*protocol.Session, error)
}

type roomService interface {
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
	UpdateConversationTitle(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
}

type eventBroadcaster interface {
	BroadcastEvent(context.Context, string, protocol.EventMessage) []error
}

// Service 负责按首条用户消息异步生成会话标题。
type Service struct {
	providers providerResolver
	sessions  sessionService
	rooms     roomService
	events    eventBroadcaster
	logger    *slog.Logger
	client    *http.Client

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
) *Service {
	return &Service{
		providers: providers,
		sessions:  sessions,
		rooms:     rooms,
		events:    events,
		logger:    logx.NewDiscardLogger(),
		client: &http.Client{
			Timeout: titleRequestTimeout,
		},
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
	if s == nil || s.providers == nil || s.client == nil {
		return
	}
	if strings.TrimSpace(request.Content) == "" || !request.hasTarget() || !request.shouldGenerateTitle() {
		return
	}
	targetKey := request.targetKey()
	if targetKey == "" {
		return
	}
	if !s.markInflight(targetKey) {
		return
	}

	asyncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), titleRequestTimeout)
	s.runAsync(func() {
		defer cancel()
		defer s.clearInflight(targetKey)
		s.generateAndApply(asyncCtx, request)
	})
}

func (s *Service) generateAndApply(ctx context.Context, request Request) {
	sessionEligible := false
	if request.shouldCheckSessionTitle() {
		ok, err := s.canAutoUpdateSession(ctx, request.SessionKey)
		if err != nil {
			s.logger.Warn("检查 session 标题状态失败",
				"session_key", request.SessionKey,
				"err", err,
			)
		} else {
			sessionEligible = ok
		}
	}
	conversationEligible := false
	resolvedRoomID := strings.TrimSpace(request.ConversationRoomID)
	if request.shouldCheckConversationTitle() {
		ok, roomID, err := s.canAutoUpdateConversation(
			ctx,
			request.ConversationID,
			request.ConversationRoomID,
		)
		if err != nil {
			s.logger.Warn("检查 room 对话标题状态失败",
				"conversation_id", request.ConversationID,
				"room_id", request.ConversationRoomID,
				"err", err,
			)
		} else {
			conversationEligible = ok
			if roomID != "" {
				resolvedRoomID = roomID
			}
		}
	}
	if request.ConversationID != "" {
		sessionEligible = sessionEligible && conversationEligible
	}
	if !sessionEligible && !conversationEligible {
		return
	}

	title, err := s.generateTitle(ctx, request.Provider, request.Content)
	if err != nil {
		s.logger.Warn("生成会话标题失败",
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
			"provider", strings.TrimSpace(request.Provider),
			"err", err,
		)
		return
	}
	if title == "" {
		return
	}

	updated := false
	if sessionEligible {
		ok, err := s.applySessionTitle(ctx, request.SessionKey, title)
		if err != nil {
			s.logger.Warn("更新 session 标题失败",
				"session_key", request.SessionKey,
				"title", title,
				"err", err,
			)
		} else if ok {
			updated = true
		}
	}
	if conversationEligible {
		ok, err := s.applyConversationTitle(
			ctx,
			request.ConversationID,
			resolvedRoomID,
			title,
		)
		if err != nil {
			s.logger.Warn("更新 room 对话标题失败",
				"conversation_id", request.ConversationID,
				"room_id", request.ConversationRoomID,
				"title", title,
				"err", err,
			)
		} else if ok {
			updated = true
		}
	}
	if updated {
		request.ConversationRoomID = resolvedRoomID
		s.broadcastResync(ctx, request)
	}
}

func (s *Service) generateTitle(
	ctx context.Context,
	provider string,
	content string,
) (string, error) {
	runtimeConfig, err := s.providers.ResolveRuntimeConfig(ctx, provider)
	if err != nil {
		return "", err
	}
	endpoint, err := buildMessagesEndpoint(runtimeConfig.BaseURL)
	if err != nil {
		return "", err
	}
	requestPayload := anthropicMessagesRequest{
		Model:       runtimeConfig.Model,
		MaxTokens:   titleMaxTokens,
		Temperature: 0,
		System: strings.TrimSpace(`你是会话标题生成器。
请根据用户的第一条消息生成一个简短标题。
要求：
1. 用自己的话概括核心意图，不要原样复述。
2. 中文控制在 2 到 12 个字；英文控制在 2 到 6 个单词。
3. 不要使用引号、句号、冒号、emoji。
4. 只返回标题文本。`),
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: truncatePromptContent(content, 400),
			},
		},
	}
	body, err := json.Marshal(requestPayload)
	if err != nil {
		return "", err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("x-api-key", runtimeConfig.AuthToken)
	httpRequest.Header.Set("anthropic-version", "2023-06-01")

	var lastErr error
	for attempt := 1; attempt <= titleMaxAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, titleAttemptTimeout)
		title, err := s.doGenerateTitle(attemptCtx, httpRequest)
		cancel()
		if err == nil {
			return title, nil
		}
		lastErr = err
		if !shouldRetryTitleRequest(err) || attempt == titleMaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(600 * time.Millisecond):
		}
	}
	return "", lastErr
}

func (s *Service) doGenerateTitle(
	ctx context.Context,
	templateRequest *http.Request,
) (string, error) {
	httpRequest := templateRequest.Clone(ctx)
	if templateRequest.GetBody != nil {
		bodyReader, err := templateRequest.GetBody()
		if err != nil {
			return "", err
		}
		httpRequest.Body = bodyReader
	}

	response, err := s.client.Do(httpRequest)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("title api 返回异常状态: %d %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload anthropicMessagesResponse
	if err = json.Unmarshal(responseBody, &payload); err != nil {
		return "", err
	}
	title := sanitizeGeneratedTitle(payload.firstText())
	if title == "" {
		return "", errors.New("标题生成返回空结果")
	}
	return title, nil
}

func (s *Service) applySessionTitle(ctx context.Context, sessionKey string, title string) (bool, error) {
	if s.sessions == nil {
		return false, nil
	}
	current, err := s.sessions.GetSession(ctx, sessionKey)
	if err != nil {
		return false, err
	}
	if current == nil || !isDefaultSessionTitle(current.Title) {
		return false, nil
	}
	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		return false, nil
	}
	_, err = s.sessions.UpdateSessionTitle(ctx, sessionKey, nextTitle)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) canAutoUpdateSession(ctx context.Context, sessionKey string) (bool, error) {
	if s.sessions == nil {
		return false, nil
	}
	current, err := s.sessions.GetSession(ctx, sessionKey)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, nil
	}
	return isDefaultSessionTitle(current.Title), nil
}

func (s *Service) applyConversationTitle(
	ctx context.Context,
	conversationID string,
	roomID string,
	title string,
) (bool, error) {
	if s.rooms == nil {
		return false, nil
	}
	current, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return false, err
	}
	if current == nil || !isDefaultConversationTitle(current.Conversation.Title, current.Room.Name) {
		return false, nil
	}
	resolvedRoomID := strings.TrimSpace(roomID)
	if resolvedRoomID == "" {
		resolvedRoomID = current.Room.ID
	}
	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		return false, nil
	}
	_, err = s.rooms.UpdateConversationTitle(ctx, resolvedRoomID, conversationID, nextTitle)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) canAutoUpdateConversation(
	ctx context.Context,
	conversationID string,
	roomID string,
) (bool, string, error) {
	if s.rooms == nil {
		return false, "", nil
	}
	current, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return false, "", err
	}
	if current == nil {
		return false, "", nil
	}
	resolvedRoomID := strings.TrimSpace(roomID)
	if resolvedRoomID == "" {
		resolvedRoomID = current.Room.ID
	}
	return isDefaultConversationTitle(current.Conversation.Title, current.Room.Name), resolvedRoomID, nil
}

func (s *Service) broadcastResync(ctx context.Context, request Request) {
	if s.events == nil || strings.TrimSpace(request.SessionKey) == "" {
		return
	}
	data := map[string]any{
		"reason": "title_generated",
	}
	if roomID := strings.TrimSpace(request.ConversationRoomID); roomID != "" {
		data["room_id"] = roomID
	}
	if conversationID := strings.TrimSpace(request.ConversationID); conversationID != "" {
		data["conversation_id"] = conversationID
	}
	event := protocol.NewEvent(protocol.EventTypeSessionResyncRequired, data)
	event.SessionKey = request.SessionKey
	if len(s.events.BroadcastEvent(ctx, request.SessionKey, event)) > 0 {
		s.logger.Warn("广播 session_resync_required 失败",
			"session_key", request.SessionKey,
			"conversation_id", request.ConversationID,
		)
	}
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

func (r Request) hasTarget() bool {
	return strings.TrimSpace(r.SessionKey) != "" || strings.TrimSpace(r.ConversationID) != ""
}

func (r Request) targetKey() string {
	if conversationID := strings.TrimSpace(r.ConversationID); conversationID != "" {
		return "conversation:" + conversationID
	}
	if sessionKey := strings.TrimSpace(r.SessionKey); sessionKey != "" {
		return "session:" + sessionKey
	}
	return ""
}

func (r Request) shouldGenerateTitle() bool {
	return r.shouldCheckSessionTitle() || r.shouldCheckConversationTitle()
}

func (r Request) shouldCheckSessionTitle() bool {
	return strings.TrimSpace(r.SessionKey) != "" &&
		r.SessionMessageCount == 0
}

func (r Request) shouldCheckConversationTitle() bool {
	return strings.TrimSpace(r.ConversationID) != "" &&
		r.ConversationMessageCount == 0
}

func isDefaultSessionTitle(title string) bool {
	normalized := strings.TrimSpace(title)
	return normalized == "" || normalized == "New Chat"
}

func isDefaultConversationTitle(title string, roomName string) bool {
	normalizedTitle := strings.TrimSpace(title)
	if normalizedTitle == "" {
		return true
	}
	normalizedRoomName := strings.TrimSpace(roomName)
	if normalizedRoomName != "" && normalizedTitle == normalizedRoomName {
		return true
	}
	return defaultConversationPattern.MatchString(normalizedTitle)
}

func truncatePromptContent(content string, maxRunes int) string {
	normalized := strings.TrimSpace(content)
	if normalized == "" || maxRunes <= 0 {
		return normalized
	}
	if utf8.RuneCountInString(normalized) <= maxRunes {
		return normalized
	}
	runes := []rune(normalized)
	return string(runes[:maxRunes])
}

func sanitizeGeneratedTitle(raw string) string {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return ""
	}
	normalized = strings.Split(normalized, "\n")[0]
	normalized = whitespacePattern.ReplaceAllString(strings.TrimSpace(normalized), " ")
	normalized = strings.Trim(normalized, "\"'“”‘’`[]()（）{}<>《》。、，！？!?:：；;")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return ""
	}
	if utf8.RuneCountInString(normalized) > 24 {
		normalized = string([]rune(normalized)[:24])
	}
	return strings.TrimSpace(normalized)
}

func buildMessagesEndpoint(baseURL string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return "", errors.New("provider base_url 不能为空")
	}
	switch {
	case strings.HasSuffix(trimmed, "/v1/messages"):
		return trimmed, nil
	case strings.HasSuffix(trimmed, "/v1"):
		return trimmed + "/messages", nil
	default:
		return trimmed + "/v1/messages", nil
	}
}

func shouldRetryTitleRequest(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "deadline exceeded") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "unexpected eof")
}

type anthropicMessagesRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicMessagesResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (r anthropicMessagesResponse) firstText() string {
	for _, item := range r.Content {
		if strings.TrimSpace(item.Type) == "text" && strings.TrimSpace(item.Text) != "" {
			return item.Text
		}
	}
	return ""
}
