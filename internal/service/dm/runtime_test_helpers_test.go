package dm

import (
	"context"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type fakeDMClient struct {
	mu               sync.Mutex
	sessionID        string
	messages         chan sdkprotocol.ReceivedMessage
	interruptCalls   int
	interruptReasons []string
	connectCalls     int
	disconnectCalls  int
	interruptErrors  []error
	disconnectErrs   []error
	connectErrors    []error
	queryErrors      []error
	queryPrompts     []string
	removeMessages   [][]string
	sentContents     []string
	queryOptions     []sdkprotocol.OutboundMessageOptions
	reconfigureOps   []agentclient.Options
	hookResponseAck  bool
	onQuery          func(context.Context, string)
	onInterrupt      func(context.Context)
}

type fakeTokenUsageRecorder struct {
	inputs []usagesvc.RecordInput
}

func (r *fakeTokenUsageRecorder) RecordMessageUsage(_ context.Context, input usagesvc.RecordInput) error {
	r.inputs = append(r.inputs, input)
	return nil
}

type externalReplyCall struct {
	agentID string
	text    string
	target  ExternalReplyTarget
}

type fakeExternalReplyDispatcher struct {
	mu          sync.Mutex
	calls       []externalReplyCall
	typingCalls []externalTypingCall
	result      ExternalReplyResult
}

type externalTypingCall struct {
	agentID string
	target  ExternalReplyTarget
	active  bool
}

func (d *fakeExternalReplyDispatcher) DeliverExternalReply(
	_ context.Context,
	agentID string,
	text string,
	target ExternalReplyTarget,
) (ExternalReplyResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, externalReplyCall{
		agentID: agentID,
		text:    text,
		target:  target,
	})
	return d.result, nil
}

func (d *fakeExternalReplyDispatcher) callsSnapshot() []externalReplyCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]externalReplyCall(nil), d.calls...)
}

func (d *fakeExternalReplyDispatcher) SetExternalTyping(
	_ context.Context,
	agentID string,
	target ExternalReplyTarget,
	active bool,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.typingCalls = append(d.typingCalls, externalTypingCall{
		agentID: agentID,
		target:  target,
		active:  active,
	})
	return nil
}

func (d *fakeExternalReplyDispatcher) typingCallsSnapshot() []externalTypingCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]externalTypingCall(nil), d.typingCalls...)
}

func newFakeDMClient() *fakeDMClient {
	return &fakeDMClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 16),
	}
}

func (c *fakeDMClient) Connect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectCalls++
	if len(c.connectErrors) == 0 {
		return nil
	}
	err := c.connectErrors[0]
	c.connectErrors = c.connectErrors[1:]
	return err
}

func (c *fakeDMClient) Query(ctx context.Context, prompt string) error {
	return c.QueryWithOptions(ctx, prompt, sdkprotocol.OutboundMessageOptions{})
}

func (c *fakeDMClient) QueryWithOptions(ctx context.Context, prompt string, options sdkprotocol.OutboundMessageOptions) error {
	if c.onQuery != nil {
		c.onQuery(ctx, prompt)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queryPrompts = append(c.queryPrompts, prompt)
	c.queryOptions = append(c.queryOptions, options)
	if len(c.queryErrors) > 0 {
		err := c.queryErrors[0]
		c.queryErrors = c.queryErrors[1:]
		return err
	}
	return ctx.Err()
}

func (c *fakeDMClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeDMClient) SendContent(_ context.Context, content any, _ *string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sentContents = append(c.sentContents, normalizeTestString(content))
	return nil
}

func (c *fakeDMClient) Interrupt(ctx context.Context) error {
	return c.interrupt(ctx, "")
}

func (c *fakeDMClient) InterruptWithReason(ctx context.Context, reason string) error {
	return c.interrupt(ctx, reason)
}

func (c *fakeDMClient) interrupt(ctx context.Context, reason string) error {
	c.mu.Lock()
	c.interruptCalls++
	c.interruptReasons = append(c.interruptReasons, reason)
	if len(c.interruptErrors) > 0 {
		err := c.interruptErrors[0]
		c.interruptErrors = c.interruptErrors[1:]
		c.mu.Unlock()
		return err
	}
	callback := c.onInterrupt
	c.mu.Unlock()
	if callback != nil {
		callback(ctx)
	}
	return nil
}

func (c *fakeDMClient) StopTask(context.Context, string) error { return nil }

func (c *fakeDMClient) SendTaskMessage(context.Context, string, string, string) error { return nil }

func (c *fakeDMClient) RemoveMessages(_ context.Context, uuids []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeMessages = append(c.removeMessages, append([]string(nil), uuids...))
	return nil
}

func (c *fakeDMClient) SetPermissionMode(context.Context, sdkpermission.Mode) error { return nil }

func (c *fakeDMClient) Disconnect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnectCalls++
	if len(c.disconnectErrs) > 0 {
		err := c.disconnectErrs[0]
		c.disconnectErrs = c.disconnectErrs[1:]
		return err
	}
	return nil
}

func (c *fakeDMClient) Reconfigure(_ context.Context, options agentclient.Options) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconfigureOps = append(c.reconfigureOps, options)
	return nil
}

func (c *fakeDMClient) Supports(capability agentclient.Capability) bool {
	return c.hookResponseAck && capability == agentclient.CapabilityHookResponseAck
}

func (c *fakeDMClient) SessionID() string { return c.sessionID }

func normalizeTestString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

type fakeDMFactory struct {
	mu      sync.Mutex
	client  *fakeDMClient
	clients []*fakeDMClient
	options []agentclient.Options
}

func (f *fakeDMFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options = append(f.options, options)
	if len(f.clients) > 0 {
		client := f.clients[0]
		f.clients = f.clients[1:]
		return client
	}
	return f.client
}

func (f *fakeDMFactory) LastOptions() agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.options) == 0 {
		return agentclient.Options{}
	}
	return f.options[len(f.options)-1]
}

func (f *fakeDMFactory) OptionAt(index int) agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.options) {
		return agentclient.Options{}
	}
	return f.options[index]
}

type fakeDMRoomSessionStore struct {
	mu      sync.Mutex
	updates []fakeDMRoomSessionUpdate
}

type fakeDMRoomSessionUpdate struct {
	roomSessionID string
	sdkSessionID  string
}

func (s *fakeDMRoomSessionStore) GetRoomSessionByKey(
	context.Context,
	string,
	protocol.SessionKey,
) (*protocol.Session, error) {
	return nil, nil
}

func (s *fakeDMRoomSessionStore) UpdateRoomSessionSDKSessionID(
	_ context.Context,
	roomSessionID string,
	sdkSessionID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, fakeDMRoomSessionUpdate{
		roomSessionID: strings.TrimSpace(roomSessionID),
		sdkSessionID:  strings.TrimSpace(sdkSessionID),
	})
	return nil
}

func (s *fakeDMRoomSessionStore) Updates() []fakeDMRoomSessionUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]fakeDMRoomSessionUpdate(nil), s.updates...)
}
