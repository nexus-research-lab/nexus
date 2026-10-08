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
