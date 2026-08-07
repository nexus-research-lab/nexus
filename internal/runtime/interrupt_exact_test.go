package runtime

import (
	"context"
	"errors"
	"testing"
)

type exactInterruptTestClient struct {
	*fakeRuntimeClient
	interruptCalls int
	interruptErr   error
	onInterrupt    func()
	entered        chan struct{}
	release        chan struct{}
}

func (c *exactInterruptTestClient) Interrupt(context.Context) error {
	c.interruptCalls++
	if c.entered != nil {
		close(c.entered)
	}
	if c.release != nil {
		<-c.release
	}
	if c.onInterrupt != nil {
		c.onInterrupt()
	}
	return c.interruptErr
}

func attachExactInterruptClient(
	manager *Manager,
	sessionKey string,
	client Client,
) {
	manager.mu.Lock()
	manager.ensureStateLocked(sessionKey).Client = client
	manager.mu.Unlock()
}

func TestManagerInterruptRoundCancelsOnlyExactOldRound(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:worker:ws:dm:cancel-exact"
	client := &exactInterruptTestClient{fakeRuntimeClient: &fakeRuntimeClient{}}
	attachExactInterruptClient(manager, sessionKey, client)
	oldCancelled := false
	successorCancelled := false
	if err := manager.StartRound(context.Background(), sessionKey, "round-old", func() {
		oldCancelled = true
		manager.MarkRoundFinished(sessionKey, "round-old")
	}); err != nil {
		t.Fatalf("failed to start old round: %v", err)
	}
	if err := manager.StartRound(context.Background(), sessionKey, "round-successor", func() {
		successorCancelled = true
		manager.MarkRoundFinished(sessionKey, "round-successor")
	}); err != nil {
		t.Fatalf("failed to start successor round: %v", err)
	}

	result, err := manager.InterruptRound(
		context.Background(),
		sessionKey,
		"round-old",
		"execution superseded",
	)
	if err != nil ||
		result.Outcome != ExactRoundLocalCancelled ||
		result.LimitationCode != "provider_interrupt_unsafe_shared_session" {
		t.Fatalf("exact interrupt = %+v, err=%v", result, err)
	}
	if !oldCancelled || successorCancelled {
		t.Fatalf(
			"oldCancelled=%t successorCancelled=%t",
			oldCancelled,
			successorCancelled,
		)
	}
	if running := manager.GetRunningRoundIDs(sessionKey); len(running) != 1 ||
		running[0] != "round-successor" {
		t.Fatalf("running rounds after exact interrupt = %+v", running)
	}
	if client.interruptCalls != 0 {
		t.Fatalf("shared provider interrupt calls = %d, want 0", client.interruptCalls)
	}
	result, err = manager.InterruptRound(
		context.Background(),
		sessionKey,
		"round-old",
		"retry",
	)
	if err != nil || result.Outcome != ExactRoundAlreadyEnded {
		t.Fatalf("already-ended retry = %+v, err=%v", result, err)
	}
	manager.MarkRoundFinished(sessionKey, "round-successor")
}

func TestManagerInterruptRoundUsesProviderOnlyForSoleRunningRound(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:worker:ws:dm:cancel-provider"
	localCancelled := false
	if err := manager.StartRound(context.Background(), sessionKey, "round-old", func() {
		localCancelled = true
		manager.MarkRoundFinished(sessionKey, "round-old")
	}); err != nil {
		t.Fatalf("failed to start target: %v", err)
	}
	client := &exactInterruptTestClient{fakeRuntimeClient: &fakeRuntimeClient{}}
	client.onInterrupt = func() {
		manager.MarkRoundFinished(sessionKey, "round-old")
	}
	attachExactInterruptClient(manager, sessionKey, client)

	result, err := manager.InterruptRound(
		context.Background(),
		sessionKey,
		"round-old",
		"execution cancelled",
	)
	if err != nil || result.Outcome != ExactRoundProviderInterrupted {
		t.Fatalf("provider interrupt = %+v, err=%v", result, err)
	}
	if client.interruptCalls != 1 || localCancelled {
		t.Fatalf(
			"provider calls=%d localCancelled=%t",
			client.interruptCalls,
			localCancelled,
		)
	}
}

func TestManagerInterruptRoundRecordsLocalFallbackAfterProviderFailure(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:worker:ws:dm:cancel-provider-failure"
	localCancelled := false
	if err := manager.StartRound(context.Background(), sessionKey, "round-old", func() {
		localCancelled = true
		manager.MarkRoundFinished(sessionKey, "round-old")
	}); err != nil {
		t.Fatalf("failed to start target: %v", err)
	}
	client := &exactInterruptTestClient{
		fakeRuntimeClient: &fakeRuntimeClient{},
		interruptErr:      errors.New("provider interrupt failed"),
	}
	attachExactInterruptClient(manager, sessionKey, client)

	result, err := manager.InterruptRound(
		context.Background(),
		sessionKey,
		"round-old",
		"execution cancelled",
	)
	if err != nil ||
		result.Outcome != ExactRoundLocalCancelled ||
		result.LimitationCode != "provider_interrupt_failed" {
		t.Fatalf("provider fallback = %+v, err=%v", result, err)
	}
	if client.interruptCalls != 1 || !localCancelled {
		t.Fatalf(
			"provider calls=%d localCancelled=%t",
			client.interruptCalls,
			localCancelled,
		)
	}
}

func TestManagerInterruptRoundRefusesUnsafeProviderFallbackWithoutLocalCancel(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:worker:ws:dm:cancel-no-local-target"
	client := &exactInterruptTestClient{fakeRuntimeClient: &fakeRuntimeClient{}}
	attachExactInterruptClient(manager, sessionKey, client)
	if err := manager.StartRound(context.Background(), sessionKey, "round-without-cancel", nil); err != nil {
		t.Fatalf("failed to start target: %v", err)
	}
	if err := manager.StartRound(context.Background(), sessionKey, "round-successor", func() {
		manager.MarkRoundFinished(sessionKey, "round-successor")
	}); err != nil {
		t.Fatalf("failed to start successor: %v", err)
	}
	result, err := manager.InterruptRound(
		context.Background(),
		sessionKey,
		"round-without-cancel",
		"execution cancelled",
	)
	if err != nil ||
		result.Outcome != ExactRoundInterruptUnsupported ||
		result.LimitationCode != "exact_local_cancel_unavailable" {
		t.Fatalf("interrupt = %+v, err=%v", result, err)
	}
	if client.interruptCalls != 0 {
		t.Fatalf("unsafe provider interrupt calls = %d", client.interruptCalls)
	}
	if running := manager.GetRunningRoundIDs(sessionKey); len(running) != 2 {
		t.Fatalf("missing exact cancel changed running rounds: %+v", running)
	}
	manager.MarkRoundFinished(sessionKey, "round-without-cancel")
	manager.MarkRoundFinished(sessionKey, "round-successor")
}

func TestManagerInterruptRoundFencesConcurrentSuccessorStart(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:worker:ws:dm:cancel-provider-race"
	if err := manager.StartRound(context.Background(), sessionKey, "round-old", func() {
		manager.MarkRoundFinished(sessionKey, "round-old")
	}); err != nil {
		t.Fatalf("failed to start target: %v", err)
	}
	client := &exactInterruptTestClient{
		fakeRuntimeClient: &fakeRuntimeClient{},
		entered:           make(chan struct{}),
		release:           make(chan struct{}),
	}
	client.onInterrupt = func() {
		manager.MarkRoundFinished(sessionKey, "round-old")
	}
	attachExactInterruptClient(manager, sessionKey, client)
	resultCh := make(chan ExactRoundInterruptResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := manager.InterruptRound(
			context.Background(),
			sessionKey,
			"round-old",
			"execution superseded",
		)
		resultCh <- result
		errCh <- err
	}()
	<-client.entered
	successorRejected := false
	if err := manager.StartRound(context.Background(), sessionKey, "round-successor", func() {
		successorRejected = true
	}); err == nil {
		t.Fatal("successor started during provider interrupt fence")
	} else if !errors.Is(err, ErrRuntimeProviderInterruptInProgress) {
		t.Fatalf("successor rejected with unexpected error: %v", err)
	}
	close(client.release)
	result := <-resultCh
	if err := <-errCh; err != nil ||
		result.Outcome != ExactRoundProviderInterrupted {
		t.Fatalf("provider result = %+v, err=%v", result, err)
	}
	if !successorRejected {
		t.Fatal("rejected successor context was not cancelled")
	}
}
