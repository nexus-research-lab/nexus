package computeruse

import (
	"context"
	"errors"
	"testing"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

type revocationClient struct {
	fakeClient
	started chan struct{}
	revoked chan struct{}
}

func (c *revocationClient) PerformAction(context.Context, nexuscua.PerformActionInput, time.Duration) (nexuscua.ActionOutput, *nexuscua.ActionRequest, error) {
	close(c.started)
	select {
	case <-c.revoked:
		return nexuscua.ActionOutput{}, nil, &nexuscua.CUAError{Code: nexuscua.ErrorSessionUnavailable}
	case <-time.After(2 * time.Second):
		return nexuscua.ActionOutput{}, nil, errors.New("action did not receive revocation")
	}
}

func (c *revocationClient) CloseSession(context.Context, nexuscua.SessionID, time.Duration) error {
	close(c.revoked)
	return nil
}

func TestCloseRevokesWhileActionOwnsMutationLock(t *testing.T) {
	client := &revocationClient{started: make(chan struct{}), revoked: make(chan struct{})}
	round := &Round{
		client: client, sessionID: "session", observationID: "observation",
		window:   &nexuscua.WindowSummary{WindowRef: "window"},
		deadline: time.Now().Add(time.Minute),
	}
	finished := make(chan error, 1)
	go func() {
		_, err := round.PerformAction(context.Background(), "press", ActionInput{Kind: "type_text", Text: "fixture"})
		finished <- err
	}()
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("action did not start")
	}
	if err := round.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.revoked:
	default:
		t.Fatal("close did not revoke the active session")
	}
	var unavailable *nexuscua.CUAError
	if err := <-finished; !errors.As(err, &unavailable) || unavailable.Code != nexuscua.ErrorSessionUnavailable {
		t.Fatalf("action was not interrupted by session revocation: %v", err)
	}
	if round.HasSession() {
		t.Fatal("closed round retained its session")
	}
}

type openingClient struct {
	fakeClient
	started chan struct{}
	release chan struct{}
	revoked chan nexuscua.SessionID
}

func (c *openingClient) OpenSession(context.Context, nexuscua.OpenSessionInput, time.Duration) (nexuscua.OpenSessionOutput, error) {
	close(c.started)
	<-c.release
	return nexuscua.OpenSessionOutput{SessionID: "late-session"}, nil
}

func (c *openingClient) CloseSession(_ context.Context, session nexuscua.SessionID, _ time.Duration) error {
	c.revoked <- session
	return nil
}

func TestCloseRevokesSessionReturnedDuringShutdown(t *testing.T) {
	client := &openingClient{started: make(chan struct{}), release: make(chan struct{}), revoked: make(chan nexuscua.SessionID, 1)}
	round := &Round{client: client, deadline: time.Now().Add(time.Minute)}
	selected := make(chan error, 1)
	go func() {
		_, err := round.SelectTarget(context.Background(), "Fixture", TargetOptions{}, func(TargetGrant) error { return nil })
		selected <- err
	}()
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("session opening did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- round.Close(context.Background()) }()
	deadline := time.Now().Add(time.Second)
	for {
		round.mu.Lock()
		closing := round.closed
		round.mu.Unlock()
		if closing {
			break
		}
		if time.Now().After(deadline) {
			close(client.release)
			t.Fatal("close did not revoke admission during session opening")
		}
		time.Sleep(time.Millisecond)
	}
	close(client.release)
	if err := <-selected; !errors.Is(err, ErrRoundClosed) {
		t.Fatalf("late session was admitted: %v", err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-client.revoked:
		if id != "late-session" {
			t.Fatalf("wrong session revoked: %s", id)
		}
	default:
		t.Fatal("late session leaked")
	}
}
