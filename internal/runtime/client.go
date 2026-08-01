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

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// Client 抽象出运行时需要的最小 SDK 能力，便于测试替身接入。
type Client interface {
	Connect(context.Context) error
	Query(context.Context, string) error
	ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage
	Interrupt(context.Context) error
	StopTask(context.Context, string) error
	SendTaskMessage(context.Context, string, string, string) error
	RemoveMessages(context.Context, []string) error
	SetPermissionMode(context.Context, sdkpermission.Mode) error
	// Retire 永久撤销 Manager 所有权；实现必须幂等、不能等待进程退出或回调 Manager。
	Retire()
	Disconnect(context.Context) error
	Reconfigure(context.Context, agentclient.Options) error
	SessionID() string
}

// Factory 负责创建 SDK client。
type Factory interface {
	New(agentclient.Options) Client
}

type defaultFactory struct{}

type sdkClientAdapter struct {
	mu                       sync.Mutex
	options                  agentclient.Options
	configVersion            uint64
	lifecycleVersion         uint64
	session                  *agentclient.Session
	messages                 chan sdkprotocol.ReceivedMessage
	cancel                   context.CancelFunc
	connecting               *sdkClientConnectFlight
	configuring              *sdkClientConfigFlight
	cleanup                  *sdkClientSessionCleanup
	streamErr                error
	retired                  bool
	newSession               func(context.Context, agentclient.Options) (*agentclient.Session, error)
	closeSession             func(*agentclient.Session) error
	reconfigureSession       func(context.Context, *agentclient.Session, agentclient.Options) error
	updateSessionEnvironment func(context.Context, *agentclient.Session, map[string]string) error
	setSessionPermissionMode func(context.Context, *agentclient.Session, sdkpermission.Mode) error
}

type sdkClientConnectFlight struct {
	done          chan struct{}
	cancel        context.CancelCauseFunc
	sharedFailure *sdkClientConnectFailure
}

type sdkClientConnectFailure struct {
	err              error
	configVersion    uint64
	lifecycleVersion uint64
}

type sdkClientConfigFlight struct {
	done chan struct{}
}

type sdkClientSessionCleanup struct {
	done chan struct{}
	err  error
}

func WrapSDKClient(options agentclient.Options) Client {
	return &sdkClientAdapter{options: options}
}

func (c *sdkClientAdapter) Connect(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.retired {
		c.mu.Unlock()
		return agentclient.ErrAborted
	}
	requestLifecycleVersion := c.lifecycleVersion
	c.mu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		if c.retired {
			c.mu.Unlock()
			return agentclient.ErrAborted
		}
		if c.lifecycleVersion != requestLifecycleVersion {
			c.mu.Unlock()
			return agentclient.ErrAborted
		}
		if c.session != nil {
			c.mu.Unlock()
			return nil
		}
		cleanup := c.cleanup
		if cleanup != nil {
			c.mu.Unlock()
			if err := waitSDKClientTransition(ctx, cleanup.done); err != nil {
				return err
			}
			if err := c.clearCompletedSDKClientCleanup(cleanup); err != nil {
				return err
			}
			continue
		}
		if connecting := c.connecting; connecting != nil {
			c.mu.Unlock()
			if err := waitSDKClientTransition(ctx, connecting.done); err != nil {
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
		connecting := &sdkClientConnectFlight{
			done:   make(chan struct{}),
			cancel: cancel,
		}
		c.connecting = connecting
		c.mu.Unlock()
		return c.runConnectFlight(connectCtx, requestLifecycleVersion, connecting)
	}
}

func (c *sdkClientAdapter) runConnectFlight(
	ctx context.Context,
	requestLifecycleVersion uint64,
	connecting *sdkClientConnectFlight,
) error {
	var sharedFailure *sdkClientConnectFailure
	defer func() { c.finishConnectFlight(connecting, sharedFailure) }()
	for {
		c.mu.Lock()
		if c.lifecycleVersion != requestLifecycleVersion {
			c.mu.Unlock()
			return agentclient.ErrAborted
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
				return agentclient.ErrAborted
			}
			if ownerErr := ctx.Err(); ownerErr != nil {
				return ownerErr
			}
			if configChanged {
				continue
			}
			sharedFailure = &sdkClientConnectFailure{
				err:              err,
				configVersion:    configVersion,
				lifecycleVersion: requestLifecycleVersion,
			}
			return err
		}

		pumpCtx, cancel := context.WithCancel(context.Background())
		messages := make(chan sdkprotocol.ReceivedMessage, 64)

		c.mu.Lock()
		configChanged := c.configVersion != configVersion
		invalidated := c.lifecycleVersion != requestLifecycleVersion
		if !configChanged && !invalidated && c.session == nil {
			c.session = session
			c.messages = messages
			c.cancel = cancel
			c.streamErr = nil
			c.mu.Unlock()
			go c.pumpMessages(pumpCtx, session, messages)
			return nil
		}
		cleanup := &sdkClientSessionCleanup{done: make(chan struct{})}
		c.cleanup = cleanup
		c.mu.Unlock()

		cancel()
		c.startSDKSessionCleanup(session, nil, cleanup)
		waitErr := waitSDKClientTransition(ctx, cleanup.done)
		c.mu.Lock()
		invalidated = invalidated || c.lifecycleVersion != requestLifecycleVersion
		c.mu.Unlock()
		if invalidated {
			return agentclient.ErrAborted
		}
		if waitErr != nil {
			return waitErr
		}
		if err := c.clearCompletedSDKClientCleanup(cleanup); err != nil {
			return err
		}
	}
}

func (c *sdkClientAdapter) connectFlightWaitResult(
	connecting *sdkClientConnectFlight,
	requestLifecycleVersion uint64,
) (error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retired {
		return agentclient.ErrAborted, true
	}
	if c.lifecycleVersion != requestLifecycleVersion {
		return agentclient.ErrAborted, true
	}
	failure := connecting.sharedFailure
	if failure == nil ||
		failure.lifecycleVersion != requestLifecycleVersion ||
		failure.configVersion != c.configVersion {
		return nil, false
	}
	return failure.err, true
}

func (c *sdkClientAdapter) finishConnectFlight(
	connecting *sdkClientConnectFlight,
	sharedFailure *sdkClientConnectFailure,
) {
	connecting.cancel(context.Canceled)
	c.mu.Lock()
	if sharedFailure != nil &&
		(sharedFailure.lifecycleVersion != c.lifecycleVersion ||
			sharedFailure.configVersion != c.configVersion) {
		sharedFailure = nil
	}
	connecting.sharedFailure = sharedFailure
	if c.connecting == connecting {
		c.connecting = nil
	}
	close(connecting.done)
	c.mu.Unlock()
}

// IsConnected 返回底层 SDK session 是否仍然存活。
// Manager 用它区分可复用 runtime 与已经断开的旧 client。
func (c *sdkClientAdapter) IsConnected() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session != nil
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

// ClearNextTurnContext 清除 bridge 尚未消费的单轮隐藏上下文。
func (c *sdkClientAdapter) ClearNextTurnContext(ctx context.Context) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	control := session.Control()
	// 新 bridge 提供显式清理；旧 bridge 的 SetNextTurnContext(nil) 也会清空
	// 同一个 buffer，作为兼容回退，避免 Slash 等原子输入带入上一轮上下文。
	clearer, ok := any(control).(interface {
		ClearNextTurnContext(context.Context) error
	})
	if ok {
		return clearer.ClearNextTurnContext(ctx)
	}
	return control.SetNextTurnContext(ctx, nil)
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

func (c *sdkClientAdapter) InterruptWithReason(ctx context.Context, reason string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.InterruptWithReason(ctx, reason)
}

func (c *sdkClientAdapter) StopTask(ctx context.Context, taskID string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.Control().StopTask(ctx, taskID)
}

func (c *sdkClientAdapter) SendTaskMessage(ctx context.Context, taskID string, message string, summary string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	return session.Control().SendTaskMessage(ctx, taskID, message, summary)
}

func (c *sdkClientAdapter) RemoveMessages(ctx context.Context, uuids []string) error {
	session, err := c.currentSession()
	if err != nil {
		return err
	}
	// v0.1.18 尚未暴露 remove_messages；用能力接口兼容已发布 bridge，
	// 同时让 go.work 下的新 bridge 继续走原生控制面。
	remover, ok := any(session.Control()).(interface {
		RemoveMessages(context.Context, []string) error
	})
	if !ok {
		return agentclient.ErrUnsupportedCapability
	}
	return remover.RemoveMessages(ctx, uuids)
}

func (c *sdkClientAdapter) SetPermissionMode(ctx context.Context, mode sdkpermission.Mode) error {
	normalized := normalizePermissionMode(mode)
	configuring, err := c.beginSDKClientConfiguration(ctx)
	if err != nil {
		return err
	}
	defer c.finishSDKClientConfiguration(configuring)

	c.mu.Lock()
	currentOptions := c.options
	if c.retired {
		c.mu.Unlock()
		return agentclient.ErrAborted
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
	if err := c.applySDKSessionPermissionMode(ctx, session, normalized); err != nil {
		c.rollbackSDKClientConfiguration(session, configVersion, currentOptions)
		if IsRuntimeTransportClosedError(err) {
			c.cleanupSDKSession(session, err)
		}
		return err
	}
	return c.ensureNotRetired()
}

// UpdateEnvironment 将运行期环境增量推送给 nxs，不重启当前会话。
func (c *sdkClientAdapter) UpdateEnvironment(ctx context.Context, environment map[string]string) error {
	if len(environment) == 0 {
		return nil
	}
	configuring, err := c.beginSDKClientConfiguration(ctx)
	if err != nil {
		return err
	}
	defer c.finishSDKClientConfiguration(configuring)

	delta := maps.Clone(environment)
	c.mu.Lock()
	currentOptions := c.options
	if c.retired {
		c.mu.Unlock()
		return agentclient.ErrAborted
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
		if err := c.applySDKSessionEnvironment(ctx, session, delta); err != nil {
			c.rollbackSDKClientConfiguration(session, configVersion, currentOptions)
			if IsRuntimeTransportClosedError(err) {
				c.cleanupSDKSession(session, err)
			}
			return err
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
func (c *sdkClientAdapter) beginSDKClientConfiguration(ctx context.Context) (*sdkClientConfigFlight, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		if c.retired {
			c.mu.Unlock()
			return nil, agentclient.ErrAborted
		}
		if c.configuring == nil {
			configuring := &sdkClientConfigFlight{done: make(chan struct{})}
			c.configuring = configuring
			c.mu.Unlock()
			return configuring, nil
		}
		configuring := c.configuring
		c.mu.Unlock()
		if err := waitSDKClientTransition(ctx, configuring.done); err != nil {
			return nil, err
		}
	}
}

func (c *sdkClientAdapter) ensureNotRetired() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retired {
		return agentclient.ErrAborted
	}
	return nil
}

func (c *sdkClientAdapter) finishSDKClientConfiguration(configuring *sdkClientConfigFlight) {
	c.mu.Lock()
	if c.configuring == configuring {
		c.configuring = nil
	}
	close(configuring.done)
	c.mu.Unlock()
}

func (c *sdkClientAdapter) rollbackSDKClientConfiguration(
	session *agentclient.Session,
	configVersion uint64,
	options agentclient.Options,
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

func (c *sdkClientAdapter) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	session, cancel, cleanup := c.detachCurrentSessionLocked(nil)
	connecting := c.connecting
	c.mu.Unlock()

	if connecting != nil {
		connecting.cancel(agentclient.ErrAborted)
	}
	if session != nil {
		c.startSDKSessionCleanup(session, cancel, cleanup)
	}
	if err := waitSDKClientCleanup(ctx, cleanup); err != nil {
		return err
	}
	if connecting != nil {
		if err := waitSDKClientTransition(ctx, connecting.done); err != nil {
			return err
		}
	}
	c.mu.Lock()
	latestCleanup := c.cleanup
	c.mu.Unlock()
	if latestCleanup != cleanup {
		if err := waitSDKClientCleanup(ctx, latestCleanup); err != nil {
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
func (c *sdkClientAdapter) Retire() {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.retired {
		c.mu.Unlock()
		return
	}
	c.retired = true
	session, cancel, cleanup := c.detachCurrentSessionLocked(agentclient.ErrAborted)
	connecting := c.connecting
	c.mu.Unlock()

	if connecting != nil {
		connecting.cancel(agentclient.ErrAborted)
	}
	if session != nil {
		c.startSDKSessionCleanup(session, cancel, cleanup)
	}
}

func (c *sdkClientAdapter) detachCurrentSessionLocked(
	err error,
) (*agentclient.Session, context.CancelFunc, *sdkClientSessionCleanup) {
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
	cleanup := &sdkClientSessionCleanup{done: make(chan struct{})}
	c.session = nil
	c.messages = nil
	c.cancel = nil
	c.cleanup = cleanup
	return session, cancel, cleanup
}

// DiscardUncleanSession 先原子隔离未收到 terminal result 的旧会话，再异步回收
// 其进程；同一 adapter 的 Connect 会等待回收完成，避免新旧 runtime 并发写
// 同一个 resume 会话。
func (c *sdkClientAdapter) DiscardUncleanSession() {
	c.mu.Lock()
	session, cancel, cleanup := c.detachCurrentSessionLocked(agentclient.ErrAborted)
	connecting := c.connecting
	c.mu.Unlock()

	if connecting != nil {
		connecting.cancel(agentclient.ErrAborted)
	}
	if session != nil {
		c.startSDKSessionCleanup(session, cancel, cleanup)
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

func waitSDKClientTransition(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func waitSDKClientCleanup(ctx context.Context, cleanup *sdkClientSessionCleanup) error {
	if cleanup == nil {
		return nil
	}
	return waitSDKClientTransition(ctx, cleanup.done)
}

func (c *sdkClientAdapter) clearCompletedSDKClientCleanup(cleanup *sdkClientSessionCleanup) error {
	if cleanup == nil {
		return nil
	}
	c.mu.Lock()
	if c.cleanup == cleanup {
		c.cleanup = nil
	}
	c.mu.Unlock()
	return nil
}

func (c *sdkClientAdapter) cleanupSDKSession(session *agentclient.Session, err error) bool {
	c.mu.Lock()
	if c.session != session {
		c.mu.Unlock()
		return false
	}
	detached, cancel, cleanup := c.detachCurrentSessionLocked(err)
	connecting := c.connecting
	c.mu.Unlock()
	if connecting != nil {
		connecting.cancel(agentclient.ErrAborted)
	}
	c.startSDKSessionCleanup(detached, cancel, cleanup)
	return true
}

func (c *sdkClientAdapter) startSDKSessionCleanup(
	session *agentclient.Session,
	cancel context.CancelFunc,
	cleanup *sdkClientSessionCleanup,
) {
	if cancel != nil {
		cancel()
	}
	go func() {
		cleanup.err = c.closeSDKSession(session)
		close(cleanup.done)
	}()
}

func (c *sdkClientAdapter) openSession(
	ctx context.Context,
	options agentclient.Options,
) (*agentclient.Session, error) {
	if c.newSession != nil {
		return c.newSession(ctx, options)
	}
	return agentclient.NewSession(ctx, options)
}

func (c *sdkClientAdapter) closeSDKSession(session *agentclient.Session) error {
	if c.closeSession != nil {
		return c.closeSession(session)
	}
	return closeSDKSession(session)
}

func (c *sdkClientAdapter) applySDKSessionReconfigure(
	ctx context.Context,
	session *agentclient.Session,
	options agentclient.Options,
) error {
	if c.reconfigureSession != nil {
		return c.reconfigureSession(ctx, session, options)
	}
	return session.Reconfigure(ctx, options)
}

func (c *sdkClientAdapter) applySDKSessionEnvironment(
	ctx context.Context,
	session *agentclient.Session,
	environment map[string]string,
) error {
	if c.updateSessionEnvironment != nil {
		return c.updateSessionEnvironment(ctx, session, environment)
	}
	return session.Control().UpdateEnvironment(ctx, environment)
}

func (c *sdkClientAdapter) applySDKSessionPermissionMode(
	ctx context.Context,
	session *agentclient.Session,
	mode sdkpermission.Mode,
) error {
	if c.setSessionPermissionMode != nil {
		return c.setSessionPermissionMode(ctx, session, mode)
	}
	return session.Control().SetPermissionMode(ctx, mode)
}

func (c *sdkClientAdapter) Reconfigure(ctx context.Context, options agentclient.Options) error {
	configuring, err := c.beginSDKClientConfiguration(ctx)
	if err != nil {
		return err
	}
	defer c.finishSDKClientConfiguration(configuring)

	c.mu.Lock()
	currentOptions := c.options
	if c.retired {
		c.mu.Unlock()
		return agentclient.ErrAborted
	}
	session := c.session
	if session != nil && shouldRestartForManagedGoalMCPServerSetChange(currentOptions, options) {
		c.mu.Unlock()
		return errManagedGoalMCPServerSetChanged
	}
	c.options = options
	c.configVersion++
	configVersion := c.configVersion
	c.mu.Unlock()
	if session == nil {
		return c.ensureNotRetired()
	}
	if err := c.applySDKSessionReconfigure(ctx, session, options); err != nil {
		c.rollbackSDKClientConfiguration(session, configVersion, currentOptions)
		if IsRuntimeTransportClosedError(err) {
			c.cleanupSDKSession(session, err)
		}
		return err
	}
	return c.ensureNotRetired()
}

func (c *sdkClientAdapter) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return strings.TrimSpace(c.options.Session.ResumeID)
	}
	return c.session.ID()
}

func (c *sdkClientAdapter) Supports(capability agentclient.Capability) bool {
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()
	return session != nil && session.Supports(capability)
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

func (c *sdkClientAdapter) Wait() error {
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

func (c *sdkClientAdapter) currentSession() (*agentclient.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retired {
		return nil, agentclient.ErrAborted
	}
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
	defer func() { c.cleanupSDKSession(session, readErr) }()
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

func (f defaultFactory) New(options agentclient.Options) Client {
	return WrapSDKClient(options)
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
		strings.Contains(message, "stream closed") ||
		strings.Contains(message, "file already closed") ||
		strings.Contains(message, "stdin unavailable") ||
		strings.Contains(message, "client: not connected")
}

func closeSDKSession(session *agentclient.Session) error {
	if session == nil {
		return nil
	}
	// cleanup 自身必须等到底层 transport 与 read loop 确认退出；调用方的
	// deadline 只约束等待，不取消共享回收，否则无法判断何时可安全重连。
	return session.Close(context.Background())
}
