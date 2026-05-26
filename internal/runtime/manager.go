package runtime

import (
	"context"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const interruptForceCancelDelay = 150 * time.Millisecond

var (
	// ErrNoRunningRound 表示当前 session 没有可接收排队输入的运行中 round。
	ErrNoRunningRound = errors.New("runtime session has no running round")
	// ErrStreamingInputUnsupported 表示底层 client 不支持流式排队输入。
	ErrStreamingInputUnsupported = errors.New("runtime client does not support streaming input")
)

// Client 抽象出运行时需要的最小 SDK 能力，便于测试替身接入。
type Client interface {
	Connect(context.Context) error
	Query(context.Context, string) error
	ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage
	Interrupt(context.Context) error
	Disconnect(context.Context) error
	Reconfigure(context.Context, agentclient.Options) error
	SessionID() string
}

type streamingInputClient interface {
	SendContent(context.Context, any, *string, string) error
}

type streamingInputOptionsClient interface {
	SendContentWithOptions(context.Context, any, *string, string, sdkprotocol.OutboundMessageOptions) error
}

// Factory 负责创建 SDK client。
type Factory interface {
	New(agentclient.Options) Client
}

type defaultFactory struct{}

type sdkClientAdapter struct {
	mu        sync.Mutex
	options   agentclient.Options
	session   *agentclient.Session
	messages  chan sdkprotocol.ReceivedMessage
	cancel    context.CancelFunc
	streamErr error
}

func WrapSDKClient(options agentclient.Options) Client {
	return &sdkClientAdapter{options: ensureBridgeBackend(options)}
}

func ensureBridgeBackend(options agentclient.Options) agentclient.Options {
	if options.Backend == nil {
		options.Backend = agentclient.ProcessBackend(agentclient.ProcessOptions{})
	}
	return options
}

func (c *sdkClientAdapter) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.session != nil {
		c.mu.Unlock()
		return nil
	}
	options := ensureBridgeBackend(c.options)
	c.options = options
	c.mu.Unlock()

	session, err := agentclient.NewSession(ctx, options)
	if err != nil {
		return err
	}

	pumpCtx, cancel := context.WithCancel(context.Background())
	messages := make(chan sdkprotocol.ReceivedMessage, 64)

	c.mu.Lock()
	c.session = session
	c.messages = messages
	c.cancel = cancel
	c.streamErr = nil
	c.mu.Unlock()

	go c.pumpMessages(pumpCtx, session, messages)
	return nil
}

func (c *sdkClientAdapter) Query(ctx context.Context, prompt string) error {
	return c.QueryWithOptions(ctx, prompt, sdkprotocol.OutboundMessageOptions{})
}

func (c *sdkClientAdapter) QueryWithOptions(ctx context.Context, prompt string, options sdkprotocol.OutboundMessageOptions) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	_, err = session.SendWithOptions(ctx, prompt, options)
	return err
}

func (c *sdkClientAdapter) QueryContent(ctx context.Context, content any) error {
	return c.QueryContentWithOptions(ctx, content, sdkprotocol.OutboundMessageOptions{})
}

func (c *sdkClientAdapter) QueryContentWithOptions(ctx context.Context, content any, options sdkprotocol.OutboundMessageOptions) error {
	if prompt, ok := content.(string); ok {
		return c.QueryWithOptions(ctx, prompt, options)
	}
	return c.SendContentWithOptions(ctx, content, nil, "", options)
}

func (c *sdkClientAdapter) SetNextTurnContext(ctx context.Context, blocks []ContextualInputBlock) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	sdkBlocks := make([]agentclient.InternalContextBlock, 0, len(blocks))
	for _, block := range normalizeContextualInputBlocks(blocks) {
		sdkBlocks = append(sdkBlocks, agentclient.InternalContextBlock{
			Name:     block.Name,
			Content:  block.Content,
			Priority: block.Priority,
			Metadata: cloneStringMap(block.Metadata),
		})
	}
	if len(sdkBlocks) == 0 {
		return nil
	}
	return session.Control().SetNextTurnContext(ctx, sdkBlocks)
}

func (c *sdkClientAdapter) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.messages == nil {
		closed := make(chan sdkprotocol.ReceivedMessage)
		close(closed)
		return closed
	}
	return c.messages
}

func (c *sdkClientAdapter) Interrupt(ctx context.Context) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.Interrupt(ctx)
}

func (c *sdkClientAdapter) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	session := c.session
	cancel := c.cancel
	c.session = nil
	c.messages = nil
	c.cancel = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if session == nil {
		return nil
	}
	return session.Close(ctx)
}

func (c *sdkClientAdapter) Reconfigure(ctx context.Context, options agentclient.Options) error {
	options = ensureBridgeBackend(options)
	c.mu.Lock()
	currentOptions := c.options
	session := c.session
	c.mu.Unlock()
	if session != nil {
		if err := applyRuntimeControls(ctx, session, currentOptions, options); err != nil {
			if IsRuntimeTransportClosedError(err) && c.markDisconnected(session, err) {
				closeSDKSession(session)
			}
			return err
		}
	}

	c.mu.Lock()
	c.options = options
	c.mu.Unlock()
	return nil
}

func (c *sdkClientAdapter) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return strings.TrimSpace(c.options.Session.ResumeID)
	}
	return c.session.ID()
}

func (c *sdkClientAdapter) SendContent(ctx context.Context, content any, parentToolUseID *string, sessionID string) error {
	return c.SendContentWithOptions(ctx, content, parentToolUseID, sessionID, sdkprotocol.OutboundMessageOptions{})
}

func (c *sdkClientAdapter) SendContentWithOptions(ctx context.Context, content any, parentToolUseID *string, sessionID string, options sdkprotocol.OutboundMessageOptions) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	payload := map[string]any{
		"type":               "user",
		"session_id":         firstNonEmpty(strings.TrimSpace(sessionID), session.ID(), c.SessionID()),
		"parent_tool_use_id": parentToolUseID,
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
	}
	_, err = session.SendMessageWithOptions(ctx, sdkprotocol.NewRawMessage(payload), options)
	return err
}

func (c *sdkClientAdapter) StreamError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streamErr
}

func (c *sdkClientAdapter) setStreamError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streamErr = err
}

func (c *sdkClientAdapter) markDisconnected(session *agentclient.Session, err error) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != session {
		return false
	}
	c.session = nil
	c.messages = nil
	c.cancel = nil
	c.streamErr = err
	return true
}

func (c *sdkClientAdapter) currentSession() (*agentclient.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, agentclient.ErrNotConnected
	}
	return c.session, nil
}

func (c *sdkClientAdapter) pumpMessages(
	ctx context.Context,
	session *agentclient.Session,
	messages chan<- sdkprotocol.ReceivedMessage,
) {
	var readErr error
	defer close(messages)
	defer func() {
		if c.markDisconnected(session, readErr) {
			closeSDKSession(session)
		}
	}()
	for {
		message, err := session.Recv(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
				return
			}
			readErr = err
			return
		}
		select {
		case <-ctx.Done():
			return
		case messages <- message:
		}
	}
}

func (f defaultFactory) New(options agentclient.Options) Client {
	return WrapSDKClient(options)
}

func applyRuntimeControls(
	ctx context.Context,
	session *agentclient.Session,
	currentOptions agentclient.Options,
	nextOptions agentclient.Options,
) error {
	control := session.Control()
	if nextOptions.Runtime.PermissionMode != "" &&
		nextOptions.Runtime.PermissionMode != currentOptions.Runtime.PermissionMode {
		if err := control.SetPermissionMode(ctx, nextOptions.Runtime.PermissionMode); err != nil {
			return err
		}
	}

	nextModel := strings.TrimSpace(nextOptions.Model)
	currentModel := strings.TrimSpace(currentOptions.Model)
	if nextModel != "" && nextModel != currentModel {
		if err := control.SetModel(ctx, nextModel); err != nil {
			return err
		}
	}

	nextMaxThinkingTokens := nextOptions.Runtime.MaxThinkingTokens
	if nextMaxThinkingTokens > 0 &&
		nextMaxThinkingTokens != currentOptions.Runtime.MaxThinkingTokens {
		if err := control.SetMaxThinkingTokens(ctx, nextMaxThinkingTokens); err != nil {
			return err
		}
	}
	return nil
}

type sessionState struct {
	Client                   Client
	RunningRounds            map[string]struct{}
	RoundCancels             map[string]context.CancelFunc
	RoundDone                map[string]chan struct{}
	Interruptions            map[string]string
	GoalAccountingFlushers   map[string]GoalAccountingFlush
	GoalAccountingClearers   map[string]GoalAccountingClear
	GoalAccountingActivators map[string]GoalAccountingActivate
	GuidedInputs             []GuidedInput
}

// GoalAccountingFlush 由正在运行的 round 提供，用于外部 Goal 状态变化前结算当前进度。
type GoalAccountingFlush func(context.Context) error

// GoalAccountingClear 由正在运行的 round 提供，用于 Goal 停止后关闭后续计量。
type GoalAccountingClear func()

// GoalAccountingActivate 由正在运行的 round 提供，用于 Goal 恢复 active 后重置计量基线。
type GoalAccountingActivate func(context.Context) error

// Manager 管理 session_key -> SDK client 与运行中 round。
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*sessionState
	factory  Factory
}

// NewManager 创建运行时管理器。
func NewManager() *Manager {
	return NewManagerWithFactory(defaultFactory{})
}

// NewManagerWithFactory 使用自定义 factory 创建运行时管理器。
func NewManagerWithFactory(factory Factory) *Manager {
	if factory == nil {
		factory = defaultFactory{}
	}
	return &Manager{
		sessions: make(map[string]*sessionState),
		factory:  factory,
	}
}

// GetOrCreate 获取或创建 client，并在复用时应用最新运行时配置。
func (m *Manager) GetOrCreate(ctx context.Context, sessionKey string, options agentclient.Options) (Client, error) {
	m.mu.RLock()
	state := m.sessions[sessionKey]
	var existing Client
	if state != nil {
		existing = state.Client
	}
	m.mu.RUnlock()
	if existing != nil {
		if err := existing.Reconfigure(ctx, options); err != nil {
			if IsRuntimeTransportClosedError(err) {
				return m.replaceDisconnectedClient(ctx, sessionKey, existing, options)
			}
			return nil, err
		}
		return existing, nil
	}

	m.mu.Lock()
	state = m.ensureStateLocked(sessionKey)
	if state.Client == nil {
		state.Client = m.factory.New(options)
		m.mu.Unlock()
		return state.Client, nil
	}
	client := state.Client
	m.mu.Unlock()
	if err := client.Reconfigure(ctx, options); err != nil {
		return nil, err
	}
	return client, nil
}

func (m *Manager) replaceDisconnectedClient(
	ctx context.Context,
	sessionKey string,
	stale Client,
	options agentclient.Options,
) (Client, error) {
	next := m.factory.New(options)
	m.mu.Lock()
	state := m.ensureStateLocked(sessionKey)
	if state.Client != stale {
		next = state.Client
		m.mu.Unlock()
		if next == nil {
			return nil, agentclient.ErrNotConnected
		}
		return next, nil
	}
	state.Client = next
	m.mu.Unlock()

	disconnectCtx, cancel := context.WithTimeout(context.Background(), roundIdleAbortTimeout)
	defer cancel()
	if err := stale.Disconnect(disconnectCtx); err != nil && !IsRuntimeTransportClosedError(err) {
		return nil, err
	}
	if next == nil {
		return nil, agentclient.ErrNotConnected
	}
	return next, nil
}

// IsRuntimeTransportClosedError 判断底层 SDK transport 是否已经断开。
func IsRuntimeTransportClosedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, agentclient.ErrNotConnected) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, os.ErrClosed) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "write payload failed") ||
		strings.Contains(message, "pipe has been ended") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "file already closed") ||
		strings.Contains(message, "client: not connected")
}

func closeSDKSession(session *agentclient.Session) {
	if session == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), roundIdleAbortTimeout)
	defer cancel()
	_ = session.Close(ctx)
}

// StartRound 注册运行中的 round，并记录其取消函数。
func (m *Manager) StartRound(sessionKey string, roundID string, cancel context.CancelFunc) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	state.RunningRounds[roundID] = struct{}{}
	delete(state.Interruptions, roundID)
	if cancel != nil {
		state.RoundCancels[roundID] = cancel
	}
	if _, exists := state.RoundDone[roundID]; !exists {
		state.RoundDone[roundID] = make(chan struct{})
	}
}

// MarkRoundFinished 把 round 从运行态中移除。
func (m *Manager) MarkRoundFinished(sessionKey string, roundID string) {
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		return
	}
	delete(state.RunningRounds, roundID)
	delete(state.RoundCancels, roundID)
	delete(state.Interruptions, roundID)
	delete(state.GoalAccountingFlushers, roundID)
	delete(state.GoalAccountingClearers, roundID)
	delete(state.GoalAccountingActivators, roundID)
	if len(state.RunningRounds) == 0 {
		state.GuidedInputs = nil
	}
	if done, ok := state.RoundDone[roundID]; ok {
		close(done)
		delete(state.RoundDone, roundID)
	}
}

// GetRunningRoundIDs 返回当前 session 的运行中轮次。
func (m *Manager) GetRunningRoundIDs(sessionKey string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.sessions[sessionKey]
	if !ok || len(state.RunningRounds) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(state.RunningRounds))
	for roundID := range state.RunningRounds {
		result = append(result, roundID)
	}
	sort.Strings(result)
	return result
}

// RegisterGoalAccountingFlush 注册或移除运行中 round 的 Goal accounting flush 回调。
func (m *Manager) RegisterGoalAccountingFlush(sessionKey string, roundID string, flush GoalAccountingFlush) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if flush == nil {
		delete(state.GoalAccountingFlushers, roundID)
		return
	}
	state.GoalAccountingFlushers[roundID] = flush
}

// RegisterGoalAccountingClear 注册或移除运行中 round 的 Goal accounting clear 回调。
func (m *Manager) RegisterGoalAccountingClear(sessionKey string, roundID string, clear GoalAccountingClear) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if clear == nil {
		delete(state.GoalAccountingClearers, roundID)
		return
	}
	state.GoalAccountingClearers[roundID] = clear
}

// RegisterGoalAccountingActivate 注册或移除运行中 round 的 Goal accounting active 回调。
func (m *Manager) RegisterGoalAccountingActivate(sessionKey string, roundID string, activate GoalAccountingActivate) {
	sessionKey = strings.TrimSpace(sessionKey)
	roundID = strings.TrimSpace(roundID)
	if sessionKey == "" || roundID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if state.GoalAccountingActivators == nil {
		state.GoalAccountingActivators = make(map[string]GoalAccountingActivate)
	}
	if activate == nil {
		delete(state.GoalAccountingActivators, roundID)
		return
	}
	state.GoalAccountingActivators[roundID] = activate
}

// FlushGoalAccounting 要求指定 session 的运行中 round 结算当前 Goal progress。
func (m *Manager) FlushGoalAccounting(ctx context.Context, sessionKey string) ([]string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil, nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingFlushers) == 0 {
		m.mu.RUnlock()
		return nil, nil
	}
	roundIDs := make([]string, 0, len(state.GoalAccountingFlushers))
	for roundID := range state.GoalAccountingFlushers {
		roundIDs = append(roundIDs, roundID)
	}
	sort.Strings(roundIDs)
	flushers := make([]GoalAccountingFlush, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		flushers = append(flushers, state.GoalAccountingFlushers[roundID])
	}
	m.mu.RUnlock()

	var firstErr error
	flushed := make([]string, 0, len(roundIDs))
	for index, flush := range flushers {
		if flush == nil {
			continue
		}
		if err := flush(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		flushed = append(flushed, roundIDs[index])
	}
	return flushed, firstErr
}

// ClearGoalAccounting 要求指定 session 的运行中 round 停止把后续 usage 归属到当前 Goal。
func (m *Manager) ClearGoalAccounting(sessionKey string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingClearers) == 0 {
		m.mu.RUnlock()
		return nil
	}
	roundIDs := make([]string, 0, len(state.GoalAccountingClearers))
	for roundID := range state.GoalAccountingClearers {
		roundIDs = append(roundIDs, roundID)
	}
	sort.Strings(roundIDs)
	clearers := make([]GoalAccountingClear, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		clearers = append(clearers, state.GoalAccountingClearers[roundID])
	}
	m.mu.RUnlock()

	cleared := make([]string, 0, len(roundIDs))
	for index, clear := range clearers {
		if clear == nil {
			continue
		}
		clear()
		cleared = append(cleared, roundIDs[index])
	}
	return cleared
}

// ActivateGoalAccounting 要求指定 session 的运行中 round 从当前快照开始归属 Goal usage。
func (m *Manager) ActivateGoalAccounting(ctx context.Context, sessionKey string) ([]string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil, nil
	}
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || len(state.GoalAccountingActivators) == 0 {
		m.mu.RUnlock()
		return nil, nil
	}
	roundIDs := make([]string, 0, len(state.GoalAccountingActivators))
	for roundID := range state.GoalAccountingActivators {
		roundIDs = append(roundIDs, roundID)
	}
	sort.Strings(roundIDs)
	activators := make([]GoalAccountingActivate, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		activators = append(activators, state.GoalAccountingActivators[roundID])
	}
	m.mu.RUnlock()

	var firstErr error
	activated := make([]string, 0, len(roundIDs))
	for index, activate := range activators {
		if activate == nil {
			continue
		}
		if err := activate(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		activated = append(activated, roundIDs[index])
	}
	return activated, firstErr
}

// CountRunningRounds 统计指定 Agent 当前活跃 round 数量。
func (m *Manager) CountRunningRounds(agentID string) int {
	if agentID == "" {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for sessionKey, state := range m.sessions {
		if len(state.RunningRounds) == 0 {
			continue
		}
		if !sessionBelongsToAgent(sessionKey, agentID) {
			continue
		}
		total += len(state.RunningRounds)
	}
	return total
}

// InterruptSession 中断当前 session 的全部运行中 round。
func (m *Manager) InterruptSession(ctx context.Context, sessionKey string, reason string) ([]string, error) {
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok {
		m.mu.RUnlock()
		return nil, nil
	}

	roundIDs := make([]string, 0, len(state.RunningRounds))
	doneSignals := make([]chan struct{}, 0, len(state.RoundDone))
	cancels := make([]context.CancelFunc, 0, len(state.RoundCancels))
	for roundID := range state.RunningRounds {
		roundIDs = append(roundIDs, roundID)
	}
	for _, roundID := range roundIDs {
		if done, ok := state.RoundDone[roundID]; ok && done != nil {
			doneSignals = append(doneSignals, done)
		}
		if cancel, ok := state.RoundCancels[roundID]; ok && cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	client := state.Client
	m.mu.RUnlock()

	sort.Strings(roundIDs)
	if len(roundIDs) == 0 {
		return nil, nil
	}

	interruptReason := strings.TrimSpace(reason)

	m.mu.Lock()
	state = m.ensureStateLocked(sessionKey)
	for _, roundID := range roundIDs {
		state.Interruptions[roundID] = interruptReason
	}
	client = state.Client
	m.mu.Unlock()

	if client == nil {
		for _, cancel := range cancels {
			cancel()
		}
		if err := waitRoundDoneSignals(ctx, doneSignals, nil); err != nil {
			return roundIDs, err
		}
		return roundIDs, nil
	}
	if err := client.Interrupt(ctx); err != nil {
		return roundIDs, err
	}
	if err := waitRoundDoneSignals(ctx, doneSignals, func() {
		for _, cancel := range cancels {
			cancel()
		}
	}); err != nil {
		return roundIDs, err
	}
	return roundIDs, nil
}

// SendContentToRunningRound 把新输入排入当前运行中的 SDK 流。
func (m *Manager) SendContentToRunningRound(ctx context.Context, sessionKey string, content any) ([]string, error) {
	m.mu.RLock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil || state.Client == nil || len(state.RunningRounds) == 0 {
		m.mu.RUnlock()
		return nil, ErrNoRunningRound
	}
	roundIDs := make([]string, 0, len(state.RunningRounds))
	for roundID := range state.RunningRounds {
		roundIDs = append(roundIDs, roundID)
	}
	client := state.Client
	m.mu.RUnlock()

	sort.Strings(roundIDs)
	if err := SendClientContent(ctx, client, content); err != nil {
		return roundIDs, err
	}
	return roundIDs, nil
}

// SendClientContent 通过 SDK streaming input 向活动 client 投递用户输入。
func SendClientContent(ctx context.Context, client Client, content any) error {
	return SendClientContentWithOptions(ctx, client, content, sdkprotocol.OutboundMessageOptions{})
}

// SendClientContentWithOptions 通过 SDK streaming input 投递带附加语义的用户输入。
func SendClientContentWithOptions(ctx context.Context, client Client, content any, options sdkprotocol.OutboundMessageOptions) error {
	if client == nil {
		return ErrNoRunningRound
	}
	if sender, ok := client.(streamingInputOptionsClient); ok {
		return sender.SendContentWithOptions(ctx, content, nil, "", options)
	}
	sender, ok := client.(streamingInputClient)
	if !ok {
		return ErrStreamingInputUnsupported
	}
	return sender.SendContent(ctx, content, nil, "")
}

type queryContentClient interface {
	QueryContent(context.Context, any) error
}

type queryContentOptionsClient interface {
	QueryContentWithOptions(context.Context, any, sdkprotocol.OutboundMessageOptions) error
}

// QueryClientContent 通过 SDK client 启动一轮用户输入，图片等结构化输入走 content block。
func QueryClientContent(ctx context.Context, client Client, content any) error {
	return QueryClientContentWithOptions(ctx, client, content, sdkprotocol.OutboundMessageOptions{})
}

// QueryClientContentWithOptions 通过 SDK client 启动一轮带附加语义的用户输入。
func QueryClientContentWithOptions(ctx context.Context, client Client, content any, options sdkprotocol.OutboundMessageOptions) error {
	if client == nil {
		return ErrNoRunningRound
	}
	if prompt, ok := content.(string); ok {
		if sender, ok := client.(interface {
			QueryWithOptions(context.Context, string, sdkprotocol.OutboundMessageOptions) error
		}); ok {
			return sender.QueryWithOptions(ctx, prompt, options)
		}
		return client.Query(ctx, prompt)
	}
	if sender, ok := client.(queryContentOptionsClient); ok {
		return sender.QueryContentWithOptions(ctx, content, options)
	}
	sender, ok := client.(queryContentClient)
	if !ok {
		return ErrStreamingInputUnsupported
	}
	return sender.QueryContent(ctx, content)
}

// GetInterruptReason 返回 round 是否已收到显式中断请求。
func (m *Manager) GetInterruptReason(sessionKey string, roundID string) string {
	if strings.TrimSpace(sessionKey) == "" || strings.TrimSpace(roundID) == "" {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.sessions[sessionKey]
	if !ok || state == nil {
		return ""
	}
	return strings.TrimSpace(state.Interruptions[roundID])
}

// CloseSession 关闭指定 session。
func (m *Manager) CloseSession(ctx context.Context, sessionKey string) error {
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if ok {
		delete(m.sessions, sessionKey)
	}
	m.mu.Unlock()
	if !ok || state.Client == nil {
		return nil
	}
	for _, cancel := range state.RoundCancels {
		if cancel != nil {
			cancel()
		}
	}
	return state.Client.Disconnect(ctx)
}

func (m *Manager) ensureStateLocked(sessionKey string) *sessionState {
	state := m.sessions[sessionKey]
	if state == nil {
		state = &sessionState{
			RunningRounds:            make(map[string]struct{}),
			RoundCancels:             make(map[string]context.CancelFunc),
			RoundDone:                make(map[string]chan struct{}),
			Interruptions:            make(map[string]string),
			GoalAccountingFlushers:   make(map[string]GoalAccountingFlush),
			GoalAccountingClearers:   make(map[string]GoalAccountingClear),
			GoalAccountingActivators: make(map[string]GoalAccountingActivate),
		}
		m.sessions[sessionKey] = state
	}
	return state
}

func sessionBelongsToAgent(sessionKey string, agentID string) bool {
	return strings.HasPrefix(sessionKey, "agent:"+agentID+":")
}

func waitRoundDoneSignals(
	ctx context.Context,
	doneSignals []chan struct{},
	forceCancel func(),
) error {
	if len(doneSignals) == 0 {
		return nil
	}

	timer := time.NewTimer(interruptForceCancelDelay)
	defer timer.Stop()
	forceCancelled := forceCancel == nil
	for _, done := range doneSignals {
		for {
			if forceCancelled {
				select {
				case <-done:
					goto nextDone
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			select {
			case <-done:
				goto nextDone
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				forceCancel()
				forceCancelled = true
			}
		}
	nextDone:
	}
	return nil
}
