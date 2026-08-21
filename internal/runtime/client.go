// INPUT: SDK bridge client、会话控制请求与子进程关闭态错误。
// OUTPUT: Nexus runtime 所需的最小 Client 能力和稳定的连接失败、换代、关闭语义。
// POS: runtime Manager 与具体 SDK bridge 之间的适配边界。
package runtime

import (
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"strings"
	"sync"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// Client 抽象出宿主管理 Agent runtime 所需的最小能力，便于测试替身接入。
type Client interface {
	Connect(context.Context) error
	Query(context.Context, string) error
	ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage
	Interrupt(context.Context) error
	StopTask(context.Context, string) error
	SendTaskMessage(context.Context, string, string, string) error
	RemoveMessages(context.Context, []string) error
	SetPermissionMode(context.Context, sdkpermission.Mode) error
	// Retire 永久撤销 Manager 所有权；实现必须幂等、立即取消连接与配置 RPC，
	// 且不能等待进程退出或回调 Manager。
	Retire()
	Disconnect(context.Context) error
	Reconfigure(context.Context, bridge.Options) error
	SessionID() string
}

// SupportsMessageExecutionPolicy 判断 runtime 是否保证消息级工具隔离与输出预算。
func SupportsMessageExecutionPolicy(client Client) bool {
	capable, ok := client.(interface {
		Supports(bridge.Capability) bool
	})
	return ok && capable.Supports(bridge.CapabilityMessageExecutionPolicy)
}

// Factory 负责创建 Agent client。
type Factory interface {
	New(bridge.Options) Client
}

type defaultFactory struct{}

// agentClient 将 bridge Session 包装成可并发复用的宿主 client。
//
// 状态不变量：
//   - session 与 cleanup 互斥；重连必须等待旧进程完全退出。
//   - lifecycleVersion 在 session 被隔离时递增，阻止旧 Connect 挂回过期进程。
//   - configVersion 标记期望配置，启动途中配置变化时关闭旧产物并重试。
type agentClient struct {
	mu                       sync.Mutex
	options                  bridge.Options
	configVersion            uint64
	lifecycleVersion         uint64
	session                  *bridge.Session
	messages                 chan sdkprotocol.ReceivedMessage
	cancel                   context.CancelFunc
	connecting               *agentClientConnectFlight
	configuring              *agentClientConfigFlight
	cleanup                  *agentClientSessionCleanup
	streamErr                error
	retired                  bool
	newSession               func(context.Context, bridge.Options) (*bridge.Session, error)
	closeSession             func(*bridge.Session) error
	reconfigureSession       func(context.Context, *bridge.Session, bridge.Options) error
	updateSessionEnvironment func(context.Context, *bridge.Session, map[string]string) error
	setSessionPermissionMode func(context.Context, *bridge.Session, sdkpermission.Mode) error
}

type agentClientConnectFlight struct {
	done   chan struct{}
	cancel context.CancelCauseFunc
	// sharedFailure 只允许同一 lifecycle、同一配置的等待者复用启动错误。
	sharedFailure *agentClientConnectFailure
}

type agentClientConnectFailure struct {
	err              error
	configVersion    uint64
	lifecycleVersion uint64
}

type agentClientConfigFlight struct {
	done   chan struct{}
	ctx    context.Context
	cancel context.CancelCauseFunc
}

type agentClientSessionCleanup struct {
	done chan struct{}
	err  error
}

// NewAgentClient 创建负责并发连接、配置换代和进程回收的 Agent client。
func NewAgentClient(options bridge.Options) Client {
	return &agentClient{options: options}
}

func (c *agentClient) Connect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.retired {
		c.mu.Unlock()
		return bridge.ErrAborted
	}
	requestLifecycleVersion := c.lifecycleVersion
	c.mu.Unlock()

	// 锁内只选择当前状态，等待和进程操作都在锁外执行：复用 session、
	// 等待 cleanup、加入已有 Connect，或成为本轮唯一的启动者。
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		if c.lifecycleVersion != requestLifecycleVersion {
			c.mu.Unlock()
			return bridge.ErrAborted
		}
		if c.session != nil {
			c.mu.Unlock()
			return nil
		}
		cleanup := c.cleanup
		if cleanup != nil {
			c.mu.Unlock()
			if err := waitAgentClientTransition(ctx, cleanup.done); err != nil {
				return err
			}
			c.clearCompletedAgentClientCleanup(cleanup)
			continue
		}
		if connecting := c.connecting; connecting != nil {
			c.mu.Unlock()
			if err := waitAgentClientTransition(ctx, connecting.done); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err, terminal := c.connectFlightWaitResult(connecting, requestLifecycleVersion); terminal {
				return err
			}
			continue
		}
		connectCtx, cancel := context.WithCancelCause(ctx)
		connecting := &agentClientConnectFlight{
			done:   make(chan struct{}),
			cancel: cancel,
		}
		c.connecting = connecting
		c.mu.Unlock()
		return c.runConnectFlight(connectCtx, requestLifecycleVersion, connecting)
	}
}

func (c *agentClient) runConnectFlight(
	ctx context.Context,
	requestLifecycleVersion uint64,
	connecting *agentClientConnectFlight,
) error {
	var sharedFailure *agentClientConnectFailure
	defer func() { c.finishConnectFlight(connecting, sharedFailure) }()
	for {
		c.mu.Lock()
		if c.lifecycleVersion != requestLifecycleVersion {
			c.mu.Unlock()
			return bridge.ErrAborted
		}
		options := c.options
		configVersion := c.configVersion
		c.mu.Unlock()

		session, err := c.openSession(ctx, options)
		if err != nil {
			c.mu.Lock()
			configChanged := c.configVersion != configVersion
			invalidated := c.lifecycleVersion != requestLifecycleVersion
			c.mu.Unlock()
			if invalidated {
				return bridge.ErrAborted
			}
			if ownerErr := ctx.Err(); ownerErr != nil {
				return ownerErr
			}
			if configChanged {
				continue
			}
			sharedFailure = &agentClientConnectFailure{
				err:              err,
				configVersion:    configVersion,
				lifecycleVersion: requestLifecycleVersion,
			}
			return err
		}

		pumpCtx, cancel := context.WithCancel(context.Background())
		messages := make(chan sdkprotocol.ReceivedMessage, 64)

		// 启动期间配置可能被热更新，lifecycle 也可能被 Disconnect 换代。
		// 只有两个快照都仍有效时才能把新进程安装为当前 session。
		c.mu.Lock()
		configChanged := c.configVersion != configVersion
		invalidated := c.lifecycleVersion != requestLifecycleVersion
		if !configChanged && !invalidated {
			c.session = session
			c.messages = messages
			c.cancel = cancel
			c.streamErr = nil
			c.mu.Unlock()
			go c.pumpMessages(pumpCtx, session, messages)
			return nil
		}
		cleanup := &agentClientSessionCleanup{done: make(chan struct{})}
		c.cleanup = cleanup
		c.mu.Unlock()

		cancel()
		c.startBridgeSessionCleanup(session, nil, cleanup)
		waitErr := waitAgentClientTransition(ctx, cleanup.done)
		c.mu.Lock()
		invalidated = invalidated || c.lifecycleVersion != requestLifecycleVersion
		c.mu.Unlock()
		if invalidated {
			return bridge.ErrAborted
		}
		if waitErr != nil {
			return waitErr
		}
		c.clearCompletedAgentClientCleanup(cleanup)
	}
}

func (c *agentClient) connectFlightWaitResult(
	connecting *agentClientConnectFlight,
	requestLifecycleVersion uint64,
) (error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lifecycleVersion != requestLifecycleVersion {
		return bridge.ErrAborted, true
	}
	failure := connecting.sharedFailure
	if failure == nil ||
		failure.lifecycleVersion != requestLifecycleVersion ||
		failure.configVersion != c.configVersion {
		return nil, false
	}
	return failure.err, true
}

// finishConnectFlight 在发布结果后才唤醒等待者，保证等待者看到完整状态。
func (c *agentClient) finishConnectFlight(
	connecting *agentClientConnectFlight,
	sharedFailure *agentClientConnectFailure,
) {
	connecting.cancel(context.Canceled)
	c.mu.Lock()
	if sharedFailure != nil &&
		(sharedFailure.lifecycleVersion != c.lifecycleVersion ||
			sharedFailure.configVersion != c.configVersion) {
		sharedFailure = nil
	}
	connecting.sharedFailure = sharedFailure
	c.connecting = nil
	close(connecting.done)
	c.mu.Unlock()
}

// IsConnected 返回底层 SDK session 是否仍然存活。
// Manager 用它区分可复用 runtime 与已经断开的旧 client。
func (c *agentClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session != nil
}

func (c *agentClient) Query(ctx context.Context, prompt string) error {
	return c.QueryWithOptions(ctx, prompt, sdkprotocol.OutboundMessageOptions{})
}

func (c *agentClient) QueryWithOptions(ctx context.Context, prompt string, options sdkprotocol.OutboundMessageOptions) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	_, err = session.SendWithOptions(ctx, prompt, options)
	return err
}

func (c *agentClient) QueryContent(ctx context.Context, content any) error {
	return c.QueryContentWithOptions(ctx, content, sdkprotocol.OutboundMessageOptions{})
}

func (c *agentClient) QueryContentWithOptions(ctx context.Context, content any, options sdkprotocol.OutboundMessageOptions) error {
	if prompt, ok := content.(string); ok {
		return c.QueryWithOptions(ctx, prompt, options)
	}
	return c.SendContentWithOptions(ctx, content, nil, "", options)
}

func (c *agentClient) SetNextTurnContext(ctx context.Context, blocks []ContextualInputBlock) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	sdkBlocks := make([]bridge.InternalContextBlock, 0, len(blocks))
	for _, block := range normalizeContextualInputBlocks(blocks) {
		sdkBlocks = append(sdkBlocks, bridge.InternalContextBlock{
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

// ClearNextTurnContext 清除 bridge 尚未消费的单轮隐藏上下文。
func (c *agentClient) ClearNextTurnContext(ctx context.Context) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.Control().ClearNextTurnContext(ctx)
}

func (c *agentClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.messages == nil {
		closed := make(chan sdkprotocol.ReceivedMessage)
		close(closed)
		return closed
	}
	return c.messages
}

func (c *agentClient) Interrupt(ctx context.Context) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.Interrupt(ctx)
}

func (c *agentClient) InterruptWithReason(ctx context.Context, reason string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.InterruptWithReason(ctx, reason)
}

func (c *agentClient) StopTask(ctx context.Context, taskID string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.Control().StopTask(ctx, taskID)
}

func (c *agentClient) SendTaskMessage(ctx context.Context, taskID string, message string, summary string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.Control().SendTaskMessage(ctx, taskID, message, summary)
}

func (c *agentClient) RemoveMessages(ctx context.Context, uuids []string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.Control().RemoveMessages(ctx, uuids)
}

func (c *agentClient) SetPermissionMode(ctx context.Context, mode sdkpermission.Mode) error {
	normalized := normalizePermissionMode(mode)
	configuring, err := c.beginAgentClientConfiguration(ctx)
	if err != nil {
		return err
	}
	defer c.finishAgentClientConfiguration(configuring)

	c.mu.Lock()
	currentOptions := c.options
	if c.retired {
		c.mu.Unlock()
		return bridge.ErrAborted
	}
	nextOptions := currentOptions
	nextOptions.Runtime.PermissionMode = normalized
	c.options = nextOptions
	c.configVersion++
	configVersion := c.configVersion
	session := c.session
	c.mu.Unlock()
	if session == nil {
		return c.ensureNotRetired()
	}
	if err := c.applyBridgeSessionPermissionMode(configuring.ctx, session, normalized); err != nil {
		c.rollbackAgentClientConfiguration(session, configVersion, currentOptions)
		if IsRuntimeTransportClosedError(err) {
			c.cleanupBridgeSession(session, err)
		}
		return configuring.normalizeError(err)
	}
	return c.ensureNotRetired()
}

// UpdateEnvironment 将运行期环境增量推送给 nxs，不重启当前会话。
func (c *agentClient) UpdateEnvironment(ctx context.Context, environment map[string]string) error {
	if len(environment) == 0 {
		return nil
	}
	configuring, err := c.beginAgentClientConfiguration(ctx)
	if err != nil {
		return err
	}
	defer c.finishAgentClientConfiguration(configuring)

	delta := maps.Clone(environment)
	c.mu.Lock()
	currentOptions := c.options
	if c.retired {
		c.mu.Unlock()
		return bridge.ErrAborted
	}
	nextOptions := currentOptions
	if nextOptions.Env == nil {
		nextOptions.Env = map[string]string{}
	} else {
		nextOptions.Env = maps.Clone(nextOptions.Env)
	}
	for key, value := range delta {
		nextOptions.Env[key] = value
	}
	c.options = nextOptions
	c.configVersion++
	configVersion := c.configVersion
	session := c.session
	c.mu.Unlock()
	if session != nil {
		if err := c.applyBridgeSessionEnvironment(configuring.ctx, session, delta); err != nil {
			c.rollbackAgentClientConfiguration(session, configVersion, currentOptions)
			if IsRuntimeTransportClosedError(err) {
				c.cleanupBridgeSession(session, err)
			}
			return configuring.normalizeError(err)
		}
	}
	return c.ensureNotRetired()
}

func normalizePermissionMode(mode sdkpermission.Mode) sdkpermission.Mode {
	if strings.TrimSpace(string(mode)) == "" {
		return sdkpermission.ModeDefault
	}
	return mode
}

// 配置调用先按顺序提交期望状态，再触达当前 session。这样 Connect 只需比较
// configVersion，就能拒绝用旧配置启动的 runtime，而不必和控制 RPC 共用锁。
func (c *agentClient) beginAgentClientConfiguration(ctx context.Context) (*agentClientConfigFlight, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		if c.retired {
			c.mu.Unlock()
			return nil, bridge.ErrAborted
		}
		if c.configuring == nil {
			configCtx, cancel := context.WithCancelCause(ctx)
			configuring := &agentClientConfigFlight{
				done:   make(chan struct{}),
				ctx:    configCtx,
				cancel: cancel,
			}
			c.configuring = configuring
			c.mu.Unlock()
			return configuring, nil
		}
		configuring := c.configuring
		c.mu.Unlock()
		if err := waitAgentClientTransition(ctx, configuring.done); err != nil {
			return nil, err
		}
	}
}

func (f *agentClientConfigFlight) normalizeError(err error) error {
	// 配置 flight 的取消原因（包括 Retire）优先于底层 RPC 随后返回的 transport 错误。
	if cause := context.Cause(f.ctx); cause != nil {
		return cause
	}
	return err
}

func (c *agentClient) ensureNotRetired() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retired {
		return bridge.ErrAborted
	}
	return nil
}

func (c *agentClient) finishAgentClientConfiguration(configuring *agentClientConfigFlight) {
	c.mu.Lock()
	c.configuring = nil
	close(configuring.done)
	c.mu.Unlock()
	configuring.cancel(context.Canceled)
}

func (c *agentClient) rollbackAgentClientConfiguration(
	session *bridge.Session,
	configVersion uint64,
	options bridge.Options,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != session || c.configVersion != configVersion {
		// 旧 session 的 RPC 与生命周期换代重叠时，新代已经读取或即将读取
		// desired options；不能再用旧代失败覆盖新代配置。
		return
	}
	c.options = options
	c.configVersion++
}

// Disconnect 先在锁内隔离当前代，再在锁外取消启动并等待进程回收。
// 调用方超时只结束等待，不会中止共享 cleanup。
func (c *agentClient) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	session, cancel, cleanup := c.detachCurrentSessionLocked(nil)
	connecting := c.connecting
	c.mu.Unlock()

	if connecting != nil {
		connecting.cancel(bridge.ErrAborted)
	}
	if session != nil {
		c.startBridgeSessionCleanup(session, cancel, cleanup)
	}
	if err := waitAgentClientCleanup(ctx, cleanup); err != nil {
		return err
	}
	if connecting != nil {
		if err := waitAgentClientTransition(ctx, connecting.done); err != nil {
			return err
		}
	}
	c.mu.Lock()
	latestCleanup := c.cleanup
	c.mu.Unlock()
	if latestCleanup != cleanup {
		if err := waitAgentClientCleanup(ctx, latestCleanup); err != nil {
			return err
		}
		if latestCleanup != nil && latestCleanup.err != nil {
			return latestCleanup.err
		}
	}
	if cleanup != nil {
		return cleanup.err
	}
	return nil
}

// Retire 先永久关闭 Manager 所有权，再异步隔离当前或正在连接的 SDK 会话。
func (c *agentClient) Retire() {
	c.mu.Lock()
	if c.retired {
		c.mu.Unlock()
		return
	}
	c.retired = true
	session, cancel, cleanup := c.detachCurrentSessionLocked(bridge.ErrAborted)
	connecting := c.connecting
	configuring := c.configuring
	c.mu.Unlock()

	if connecting != nil {
		connecting.cancel(bridge.ErrAborted)
	}
	if configuring != nil {
		configuring.cancel(bridge.ErrAborted)
	}
	if session != nil {
		c.startBridgeSessionCleanup(session, cancel, cleanup)
	}
}

func (c *agentClient) detachCurrentSessionLocked(
	err error,
) (*bridge.Session, context.CancelFunc, *agentClientSessionCleanup) {
	// 即使当前没有 session 也要换代，使尚未完成的 Connect 无法回挂。
	c.lifecycleVersion++
	if c.session == nil {
		if err != nil {
			c.streamErr = err
		}
		return nil, nil, c.cleanup
	}
	c.streamErr = err
	session := c.session
	cancel := c.cancel
	cleanup := &agentClientSessionCleanup{done: make(chan struct{})}
	c.session = nil
	c.messages = nil
	c.cancel = nil
	c.cleanup = cleanup
	return session, cancel, cleanup
}

// DiscardUncleanSession 先原子隔离未收到 terminal result 的旧会话，再异步回收
// 其进程；同一 adapter 的 Connect 会等待回收完成，避免新旧 runtime 并发写
// 同一个 resume 会话。
func (c *agentClient) DiscardUncleanSession() {
	c.mu.Lock()
	session, cancel, cleanup := c.detachCurrentSessionLocked(bridge.ErrAborted)
	connecting := c.connecting
	c.mu.Unlock()

	if connecting != nil {
		connecting.cancel(bridge.ErrAborted)
	}
	if session != nil {
		c.startBridgeSessionCleanup(session, cancel, cleanup)
	}
}

// DiscardUncleanClientSession 隔离无法证明消息边界干净的 SDK 会话。
func DiscardUncleanClientSession(client Client) bool {
	discarder, ok := client.(interface{ DiscardUncleanSession() })
	if !ok {
		return false
	}
	discarder.DiscardUncleanSession()
	return true
}

func waitAgentClientTransition(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func waitAgentClientCleanup(ctx context.Context, cleanup *agentClientSessionCleanup) error {
	if cleanup == nil {
		return nil
	}
	return waitAgentClientTransition(ctx, cleanup.done)
}

// clearCompletedAgentClientCleanup 只清除自己观察到的 cleanup，不能覆盖并发产生的新代。
func (c *agentClient) clearCompletedAgentClientCleanup(cleanup *agentClientSessionCleanup) {
	c.mu.Lock()
	if c.cleanup == cleanup {
		c.cleanup = nil
	}
	c.mu.Unlock()
}

func (c *agentClient) cleanupBridgeSession(session *bridge.Session, err error) bool {
	c.mu.Lock()
	// 旧 read loop 可能晚于新 session 退出，不能让它清理当前代。
	if c.session != session {
		c.mu.Unlock()
		return false
	}
	detached, cancel, cleanup := c.detachCurrentSessionLocked(err)
	connecting := c.connecting
	c.mu.Unlock()
	if connecting != nil {
		connecting.cancel(bridge.ErrAborted)
	}
	c.startBridgeSessionCleanup(detached, cancel, cleanup)
	return true
}

func (c *agentClient) startBridgeSessionCleanup(
	session *bridge.Session,
	cancel context.CancelFunc,
	cleanup *agentClientSessionCleanup,
) {
	if cancel != nil {
		cancel()
	}
	go func() {
		cleanup.err = c.closeBridgeSession(session)
		close(cleanup.done)
	}()
}

func (c *agentClient) openSession(
	ctx context.Context,
	options bridge.Options,
) (*bridge.Session, error) {
	if c.newSession != nil {
		return c.newSession(ctx, options)
	}
	return bridge.NewSession(ctx, options)
}

func (c *agentClient) closeBridgeSession(session *bridge.Session) error {
	if c.closeSession != nil {
		return c.closeSession(session)
	}
	return closeBridgeSession(session)
}

func (c *agentClient) applyBridgeSessionReconfigure(
	ctx context.Context,
	session *bridge.Session,
	options bridge.Options,
) error {
	if c.reconfigureSession != nil {
		return c.reconfigureSession(ctx, session, options)
	}
	return session.Reconfigure(ctx, options)
}

func (c *agentClient) applyBridgeSessionEnvironment(
	ctx context.Context,
	session *bridge.Session,
	environment map[string]string,
) error {
	if c.updateSessionEnvironment != nil {
		return c.updateSessionEnvironment(ctx, session, environment)
	}
	return session.Control().UpdateEnvironment(ctx, environment)
}

func (c *agentClient) applyBridgeSessionPermissionMode(
	ctx context.Context,
	session *bridge.Session,
	mode sdkpermission.Mode,
) error {
	if c.setSessionPermissionMode != nil {
		return c.setSessionPermissionMode(ctx, session, mode)
	}
	return session.Control().SetPermissionMode(ctx, mode)
}

func (c *agentClient) Reconfigure(ctx context.Context, options bridge.Options) error {
	configuring, err := c.beginAgentClientConfiguration(ctx)
	if err != nil {
		return err
	}
	defer c.finishAgentClientConfiguration(configuring)

	c.mu.Lock()
	currentOptions := c.options
	if c.retired {
		c.mu.Unlock()
		return bridge.ErrAborted
	}
	session := c.session
	c.options = options
	c.configVersion++
	configVersion := c.configVersion
	c.mu.Unlock()
	if session == nil {
		return c.ensureNotRetired()
	}
	if err := c.applyBridgeSessionReconfigure(configuring.ctx, session, options); err != nil {
		c.rollbackAgentClientConfiguration(session, configVersion, currentOptions)
		if IsRuntimeTransportClosedError(err) {
			c.cleanupBridgeSession(session, err)
		}
		return configuring.normalizeError(err)
	}
	return c.ensureNotRetired()
}

func (c *agentClient) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return strings.TrimSpace(c.options.Session.ResumeID)
	}
	return c.session.ID()
}

func (c *agentClient) Supports(capability bridge.Capability) bool {
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()
	return session != nil && session.Supports(capability)
}

func (c *agentClient) SendContent(ctx context.Context, content any, parentToolUseID *string, sessionID string) error {
	return c.SendContentWithOptions(ctx, content, parentToolUseID, sessionID, sdkprotocol.OutboundMessageOptions{})
}

func (c *agentClient) SendContentWithOptions(ctx context.Context, content any, parentToolUseID *string, sessionID string, options sdkprotocol.OutboundMessageOptions) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	payload := map[string]any{
		"type":               "user",
		"parent_tool_use_id": parentToolUseID,
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
	}
	// 未显式指定时必须省略字段，让 bridge 按 lifecycle、配置和 resume 状态解析；
	// 空字符串会被视为已经指定，从而截断 bridge 的默认路径。
	if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
		payload["session_id"] = sessionID
	}
	_, err = session.SendMessageWithOptions(ctx, sdkprotocol.NewRawMessage(payload), options)
	return err
}

func (c *agentClient) StreamError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streamErr
}

func (c *agentClient) Wait() error {
	c.mu.Lock()
	session := c.session
	streamErr := c.streamErr
	c.mu.Unlock()
	if streamErr != nil {
		return streamErr
	}
	if session == nil {
		return nil
	}
	return session.Wait()
}

func (c *agentClient) currentSession() (*bridge.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retired {
		return nil, bridge.ErrAborted
	}
	if c.session == nil {
		return nil, bridge.ErrNotConnected
	}
	return c.session, nil
}

func (c *agentClient) pumpMessages(
	ctx context.Context,
	session *bridge.Session,
	messages chan<- sdkprotocol.ReceivedMessage,
) {
	var readErr error
	// read loop 独占消息通道的关闭；session 身份检查负责屏蔽过期 loop 的回收动作。
	defer close(messages)
	defer func() { c.cleanupBridgeSession(session, readErr) }()
	for {
		message, err := session.Recv(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				readErr = session.Wait()
				return
			}
			// 中文注释：SDK abort 是有效的 round 中断信号，不能当作普通 EOF 吞掉。
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

func (f defaultFactory) New(options bridge.Options) Client {
	return NewAgentClient(options)
}

// IsRuntimeTransportClosedError 判断底层 SDK transport 是否已经断开。
func IsRuntimeTransportClosedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, bridge.ErrNotConnected) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, os.ErrClosed) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "write payload failed") ||
		strings.Contains(message, "pipe has been ended") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "stream closed") ||
		strings.Contains(message, "file already closed") ||
		strings.Contains(message, "stdin unavailable") ||
		strings.Contains(message, "client: not connected")
}

func closeBridgeSession(session *bridge.Session) error {
	// cleanup 自身必须等到底层 transport 与 read loop 确认退出；调用方的
	// deadline 只约束等待，不取消共享回收，否则无法判断何时可安全重连。
	return session.Close(context.Background())
}
