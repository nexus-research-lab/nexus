package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func ownerRuntimeOptions(ownerUserID string) agentclient.Options {
	return agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": ownerUserID},
	}
}

func TestManagerRevokeAgentSessionsClosesOnlyMatchingAgentAndTombstones(t *testing.T) {
	deletedDM := &fakeRuntimeClient{}
	deletedRoom := &fakeRuntimeClient{}
	sibling := &fakeRuntimeClient{}
	otherOwner := &fakeRuntimeClient{}
	reaper := &fakeOwnerProcessReaper{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{
		deletedDM,
		deletedRoom,
		sibling,
		otherOwner,
	}})
	manager.SetOwnerProcessReaper(reaper)

	const (
		deletedDMKey   = "agent:agent-a:ws:dm:dm-a"
		deletedRoomKey = "agent:agent-a:ws:group:conversation-a"
		siblingKey     = "agent:agent-b:ws:dm:dm-b"
		otherOwnerKey  = "agent:agent-a:ws:dm:other-owner"
	)
	for _, input := range []struct {
		sessionKey  string
		ownerUserID string
	}{
		{deletedDMKey, "owner-a"},
		{deletedRoomKey, "owner-a"},
		{siblingKey, "owner-a"},
		{otherOwnerKey, "owner-b"},
	} {
		if _, err := manager.GetOrCreate(
			context.Background(),
			input.sessionKey,
			ownerRuntimeOptions(input.ownerUserID),
		); err != nil {
			t.Fatalf("创建 %s runtime: %v", input.sessionKey, err)
		}
	}
	roundCanceled := make(chan struct{})
	if err := manager.StartRound(t.Context(), deletedRoomKey, "room-round", func() {
		close(roundCanceled)
		manager.MarkRoundFinished(deletedRoomKey, "room-round")
	}); err != nil {
		t.Fatalf("启动待删除 Agent Room round: %v", err)
	}

	closed, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	)
	if err != nil {
		t.Fatalf("撤销 Agent runtime: %v", err)
	}
	if closed != 2 {
		t.Fatalf("撤销 session 数量=%d，want 2", closed)
	}
	select {
	case <-roundCanceled:
	default:
		t.Fatal("Agent 删除未取消运行中的 Room round")
	}
	if deletedDM.disconnectCalls != 1 || deletedRoom.disconnectCalls != 1 {
		t.Fatalf(
			"待删除 Agent runtime 未全部断开: dm=%d room=%d",
			deletedDM.disconnectCalls,
			deletedRoom.disconnectCalls,
		)
	}
	if sibling.disconnectCalls != 0 || otherOwner.disconnectCalls != 0 {
		t.Fatalf(
			"撤销越过 owner+Agent 边界: sibling=%d other_owner=%d",
			sibling.disconnectCalls,
			otherOwner.disconnectCalls,
		)
	}
	if !manager.HasSession(siblingKey) || !manager.HasSession(otherOwnerKey) {
		t.Fatal("同 owner 其他 Agent 或其他 owner 同名 Agent session 被误删")
	}
	if len(reaper.owners) != 0 {
		t.Fatalf("同 owner 仍有其他 Agent 时不得 owner 级回收: %v", reaper.owners)
	}

	if _, err = manager.GetOrCreate(
		context.Background(),
		"agent:agent-a:ws:dm:new-after-delete",
		ownerRuntimeOptions("owner-a"),
	); !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("删除墓碑未阻止新 session: %v", err)
	}
	lateRoundCanceled := false
	if err := manager.StartRound(t.Context(), deletedDMKey, "late-round", func() { lateRoundCanceled = true }); !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("删除后旧 session_key 启动错误=%v，期望 Agent 已撤销", err)
	}
	if !lateRoundCanceled {
		t.Fatal("拒绝删除后 round 时必须取消调用方 context")
	}
	if _, err = manager.GetOrCreate(
		context.Background(),
		otherOwnerKey,
		ownerRuntimeOptions("owner-b"),
	); err != nil {
		t.Fatalf("其他 owner 的同名 Agent 不应被墓碑阻断: %v", err)
	}
	if err = manager.Connect(context.Background(), siblingKey, sibling); err != nil {
		t.Fatalf("同 owner 的其他 Agent Connect 不应受影响: %v", err)
	}
	if err = manager.Connect(context.Background(), otherOwnerKey, otherOwner); err != nil {
		t.Fatalf("其他 owner 的同名 Agent Connect 不应受影响: %v", err)
	}
}

func TestManagerRevokeAgentSessionsKeepsTombstoneWhenDisconnectFails(t *testing.T) {
	disconnectErr := errors.New("runtime process did not exit")
	client := &fakeRuntimeClient{disconnectErr: disconnectErr}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	const sessionKey = "agent:agent-a:ws:dm:disconnect-failure"
	options := ownerRuntimeOptions("owner-a")
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); !errors.Is(err, disconnectErr) {
		t.Fatalf("断连错误必须交给删除 reconcile: %v", err)
	}
	if _, err := manager.GetOrCreate(
		context.Background(),
		"agent:agent-a:ws:dm:retry-after-failure",
		options,
	); !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("后置清理失败不得解除 Agent 墓碑: %v", err)
	}
}

type blockingConnectRuntimeClient struct {
	*fakeRuntimeClient
	started        chan struct{}
	release        chan struct{}
	startOnce      sync.Once
	releaseOnce    sync.Once
	disconnectCall atomic.Int32
}

func (c *blockingConnectRuntimeClient) Connect(context.Context) error {
	c.startOnce.Do(func() { close(c.started) })
	<-c.release
	return nil
}

func (c *blockingConnectRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCall.Add(1)
	c.releaseOnce.Do(func() { close(c.release) })
	return nil
}

func TestManagerConnectDisconnectsLateProcessWhenAgentIsDeleted(t *testing.T) {
	client := &blockingConnectRuntimeClient{
		fakeRuntimeClient: &fakeRuntimeClient{},
		started:           make(chan struct{}),
		release:           make(chan struct{}),
	}
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client {
		return client
	}))
	const sessionKey = "agent:agent-a:ws:dm:connect-race"
	if _, err := manager.GetOrCreate(
		context.Background(),
		sessionKey,
		ownerRuntimeOptions("owner-a"),
	); err != nil {
		t.Fatal(err)
	}

	connectResult := make(chan error, 1)
	go func() {
		connectResult <- manager.Connect(context.Background(), sessionKey, client)
	}()
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("client Connect 未进入删除竞态窗口")
	}
	if _, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); err != nil {
		t.Fatal(err)
	}
	if err := <-connectResult; !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("删除期间迟到完成的 Connect 未被拒绝: %v", err)
	}
	if client.disconnectCall.Load() != 2 {
		t.Fatalf(
			"删除撤销及 Connect 后置核验应各断开一次: disconnect=%d",
			client.disconnectCall.Load(),
		)
	}
}

type blockingReconfigureRuntimeClient struct {
	*fakeRuntimeClient
	started        chan struct{}
	release        chan struct{}
	startOnce      sync.Once
	releaseOnce    sync.Once
	block          atomic.Bool
	disconnectCall atomic.Int32
}

func (c *blockingReconfigureRuntimeClient) Reconfigure(
	context.Context,
	agentclient.Options,
) error {
	if c.block.Load() {
		c.startOnce.Do(func() { close(c.started) })
		<-c.release
	}
	return nil
}

func (c *blockingReconfigureRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCall.Add(1)
	c.releaseOnce.Do(func() { close(c.release) })
	return nil
}

func TestManagerRevokeAgentSessionsWinsExistingClientReconfigureRace(t *testing.T) {
	client := &blockingReconfigureRuntimeClient{
		fakeRuntimeClient: &fakeRuntimeClient{},
		started:           make(chan struct{}),
		release:           make(chan struct{}),
	}
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client {
		return client
	}))
	const sessionKey = "agent:agent-a:ws:dm:reconfigure-race"
	options := ownerRuntimeOptions("owner-a")
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatal(err)
	}
	client.block.Store(true)

	getResult := make(chan error, 1)
	go func() {
		_, err := manager.GetOrCreate(context.Background(), sessionKey, options)
		getResult <- err
	}()
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("existing-client Reconfigure 未进入竞态窗口")
	}
	if _, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); err != nil {
		t.Fatal(err)
	}
	if err := <-getResult; !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("并发 Reconfigure 越过删除墓碑: %v", err)
	}
	if client.disconnectCall.Load() != 1 {
		t.Fatalf("existing client disconnect=%d，want 1", client.disconnectCall.Load())
	}
}

type atomicDisconnectRuntimeClient struct {
	*fakeRuntimeClient
	disconnectCall atomic.Int32
}

func (c *atomicDisconnectRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCall.Add(1)
	return nil
}

type blockingReplacementFactory struct {
	stale            Client
	candidate        Client
	calls            atomic.Int32
	candidateStarted chan struct{}
	releaseCandidate chan struct{}
}

func (f *blockingReplacementFactory) New(agentclient.Options) Client {
	if f.calls.Add(1) == 1 {
		return f.stale
	}
	close(f.candidateStarted)
	<-f.releaseCandidate
	return f.candidate
}

func TestManagerRevokeAgentSessionsWinsReplacementCandidateRace(t *testing.T) {
	stale := &atomicDisconnectRuntimeClient{
		fakeRuntimeClient: &fakeRuntimeClient{
			reconfigureErr: &agentclient.RestartRequiredError{
				Reason: agentclient.RestartReasonProcessEnvChanged,
			},
		},
	}
	candidate := &atomicDisconnectRuntimeClient{fakeRuntimeClient: &fakeRuntimeClient{}}
	factory := &blockingReplacementFactory{
		stale:            stale,
		candidate:        candidate,
		candidateStarted: make(chan struct{}),
		releaseCandidate: make(chan struct{}),
	}
	manager := NewManagerWithFactory(factory)
	const sessionKey = "agent:agent-a:ws:dm:replacement-race"
	options := ownerRuntimeOptions("owner-a")
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatal(err)
	}

	getResult := make(chan error, 1)
	go func() {
		_, err := manager.GetOrCreate(context.Background(), sessionKey, options)
		getResult <- err
	}()
	select {
	case <-factory.candidateStarted:
	case <-time.After(time.Second):
		t.Fatal("replacement candidate 未进入竞态窗口")
	}
	if _, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); err != nil {
		t.Fatal(err)
	}
	close(factory.releaseCandidate)
	if err := <-getResult; !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("replacement candidate 越过删除墓碑: %v", err)
	}
	if stale.disconnectCall.Load() != 1 {
		t.Fatalf("stale client disconnect=%d，want 1", stale.disconnectCall.Load())
	}
	if candidate.disconnectCall.Load() != 1 {
		t.Fatalf("被墓碑拒绝的 candidate disconnect=%d，want 1", candidate.disconnectCall.Load())
	}
}

type blockingCreateFactory struct {
	candidate Client
	started   chan struct{}
	release   chan struct{}
}

func (f *blockingCreateFactory) New(agentclient.Options) Client {
	close(f.started)
	<-f.release
	return f.candidate
}

func TestManagerRevokeAgentSessionsWinsSecondLockCreateRace(t *testing.T) {
	candidate := &atomicDisconnectRuntimeClient{fakeRuntimeClient: &fakeRuntimeClient{}}
	factory := &blockingCreateFactory{
		candidate: candidate,
		started:   make(chan struct{}),
		release:   make(chan struct{}),
	}
	manager := NewManagerWithFactory(factory)
	const sessionKey = "agent:agent-a:ws:dm:create-race"
	options := ownerRuntimeOptions("owner-a")

	getResult := make(chan error, 1)
	go func() {
		_, err := manager.GetOrCreate(context.Background(), sessionKey, options)
		getResult <- err
	}()
	select {
	case <-factory.started:
	case <-time.After(time.Second):
		t.Fatal("fresh candidate 未进入 first/second lock 竞态窗口")
	}
	if closed, err := manager.RevokeAgentSessions(
		context.Background(),
		"owner-a",
		"agent-a",
	); err != nil || closed != 0 {
		t.Fatalf("无已发布 session 的撤销: closed=%d err=%v", closed, err)
	}
	close(factory.release)
	if err := <-getResult; !errors.Is(err, ErrRuntimeAgentRevoked) {
		t.Fatalf("second-lock create 越过删除墓碑: %v", err)
	}
	if candidate.disconnectCall.Load() != 1 {
		t.Fatalf("未发布 candidate disconnect=%d，want 1", candidate.disconnectCall.Load())
	}
	if manager.HasSession(sessionKey) {
		t.Fatal("second-lock 拒绝后不得发布 candidate")
	}
}
