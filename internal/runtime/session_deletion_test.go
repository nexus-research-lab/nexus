package runtime

import (
	"context"
	"errors"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestManagerSessionDeletionFenceBlocksAdmissionAndCanAbort(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:agent-a:ws:dm:delete-fence"
	options := agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatal(err)
	}
	lease, err := manager.BeginSessionDeletion(sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.GetOrCreate(
		context.Background(),
		sessionKey,
		options,
	); !errors.Is(err, ErrRuntimeSessionDeleted) {
		t.Fatalf("new runtime admission was not blocked: %v", err)
	}
	canceled := false
	if err := manager.StartRound(t.Context(), sessionKey, "late-round", func() { canceled = true }); !errors.Is(err, ErrRuntimeSessionDeleted) {
		t.Fatalf("late round error = %v, want session deleted", err)
	}
	if !canceled {
		t.Fatal("rejected round must cancel its context")
	}
	manager.AbortSessionDeletion(lease)
	if _, err = manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatalf("aborted deletion fence still blocks runtime: %v", err)
	}
}

func TestManagerSessionDeletionFenceRemainsAfterRuntimeClose(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:agent-a:ws:dm:deleted"
	options := agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.BeginSessionDeletion(sessionKey); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.GetOrCreate(
		context.Background(),
		sessionKey,
		options,
	); !errors.Is(err, ErrRuntimeSessionDeleted) {
		t.Fatalf("closed deleted session was resurrected: %v", err)
	}
}

func TestManagerSessionDeletionFenceBlocksBackgroundTaskAndLegacyAlias(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	leftKey := "agent:agent-a:ws:dm:user:one"
	rightKey := "agent:agent-a:ws:dm:user_3aone"
	if _, err := manager.BeginSessionDeletion(leftKey); err != nil {
		t.Fatal(err)
	}
	ran := false
	if manager.StartBackgroundTaskForOwner(rightKey, "owner-a", func(context.Context) {
		ran = true
	}) {
		t.Fatal("legacy collision alias started a background task through deletion fence")
	}
	if ran {
		t.Fatal("rejected background task callback executed")
	}
	if _, err := manager.GetOrCreate(
		context.Background(),
		rightKey,
		agentclient.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"}},
	); !errors.Is(err, ErrRuntimeSessionDeleted) {
		t.Fatalf("legacy collision alias runtime admission was not blocked: %v", err)
	}
}
