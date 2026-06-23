package runtime

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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
	return &sdkClientAdapter{options: options}
}

func (c *sdkClientAdapter) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.session != nil {
		c.mu.Unlock()
		return nil
	}
	options := c.options
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
	c.mu.Lock()
	currentOptions := c.options
	session := c.session
	c.mu.Unlock()
	if session != nil {
		if shouldRestartForManagedGoalMCPServerSetChange(currentOptions, options) {
			return errManagedGoalMCPServerSetChanged
		}
		if err := session.Reconfigure(ctx, options); err != nil {
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
			if errors.Is(err, context.Canceled) || errors.Is(err, agentclient.ErrAborted) || errors.Is(err, io.EOF) {
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
