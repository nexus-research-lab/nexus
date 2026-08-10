package server

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
)

type recordingAgentDeletionChannelCoordinator struct {
	mu        sync.Mutex
	events    []string
	beforeErr error
	afterErr  error
}

func (c *recordingAgentDeletionChannelCoordinator) record(event string) {
	c.mu.Lock()
	c.events = append(c.events, event)
	c.mu.Unlock()
}

func (c *recordingAgentDeletionChannelCoordinator) CoordinateAgentDeletion(
	ctx context.Context,
	_ string,
	_ string,
	deletePersistent func(context.Context) error,
) error {
	c.record("channel_before")
	if c.beforeErr != nil {
		return c.beforeErr
	}
	if err := deletePersistent(ctx); err != nil {
		return err
	}
	c.record("channel_after")
	return c.afterErr
}

type recordingAgentRuntimeRevoker struct {
	channel *recordingAgentDeletionChannelCoordinator
	err     error
	calls   int
}

func (r *recordingAgentRuntimeRevoker) RevokeAgentSessions(
	_ context.Context,
	ownerUserID string,
	agentID string,
) (int, error) {
	r.calls++
	r.channel.record("runtime:" + ownerUserID + ":" + agentID)
	return 2, r.err
}

func TestAgentDeletionCoordinatorRevokesRuntimeAtCommitBeforeChannelCleanup(t *testing.T) {
	channelErr := errors.New("channel cleanup failed")
	runtimeErr := errors.New("runtime cleanup failed")
	channels := &recordingAgentDeletionChannelCoordinator{afterErr: channelErr}
	runtimes := &recordingAgentRuntimeRevoker{channel: channels, err: runtimeErr}
	coordinator := newAgentDeletionCoordinator(channels, runtimes)

	err := coordinator.CoordinateAgentDeletion(
		context.Background(),
		"owner-a",
		"agent-a",
		func(context.Context) error {
			channels.record("persistence_commit")
			return nil
		},
	)
	if !errors.Is(err, channelErr) || !errors.Is(err, runtimeErr) {
		t.Fatalf("必须汇总 Channel/runtime 后置错误: %v", err)
	}
	channels.mu.Lock()
	events := slices.Clone(channels.events)
	channels.mu.Unlock()
	want := []string{
		"channel_before",
		"persistence_commit",
		"runtime:owner-a:agent-a",
		"channel_after",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("删除提交与撤销顺序=%v，want %v", events, want)
	}
	if runtimes.calls != 1 {
		t.Fatalf("runtime revoke calls=%d，want 1", runtimes.calls)
	}
}

func TestAgentDeletionCoordinatorDoesNotRevokeBeforePersistenceCommit(t *testing.T) {
	persistenceErr := errors.New("database conflict")
	channels := &recordingAgentDeletionChannelCoordinator{}
	runtimes := &recordingAgentRuntimeRevoker{channel: channels}
	coordinator := newAgentDeletionCoordinator(channels, runtimes)

	err := coordinator.CoordinateAgentDeletion(
		context.Background(),
		"owner-a",
		"agent-a",
		func(context.Context) error { return persistenceErr },
	)
	if !errors.Is(err, persistenceErr) {
		t.Fatalf("持久删除错误=%v，want %v", err, persistenceErr)
	}
	if runtimes.calls != 0 {
		t.Fatalf("数据库未提交不得撤销 runtime: calls=%d", runtimes.calls)
	}
}

func TestAgentDeletionCoordinatorDoesNotDeleteWhenChannelSnapshotFails(t *testing.T) {
	beforeErr := errors.New("channel impact snapshot failed")
	channels := &recordingAgentDeletionChannelCoordinator{beforeErr: beforeErr}
	runtimes := &recordingAgentRuntimeRevoker{channel: channels}
	coordinator := newAgentDeletionCoordinator(channels, runtimes)
	persistenceCalls := 0

	err := coordinator.CoordinateAgentDeletion(
		context.Background(),
		"owner-a",
		"agent-a",
		func(context.Context) error {
			persistenceCalls++
			return nil
		},
	)
	if !errors.Is(err, beforeErr) {
		t.Fatalf("Channel 前置错误=%v，want %v", err, beforeErr)
	}
	if persistenceCalls != 0 || runtimes.calls != 0 {
		t.Fatalf(
			"Channel 快照失败不得提交或撤销: persistence=%d runtime=%d",
			persistenceCalls,
			runtimes.calls,
		)
	}
}
