package session_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	"github.com/nexus-research-lab/nexus/internal/service/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestSessionDeletionRecoveryCommitsCrashInterruptedDelete(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	agentValue, err := agentService.CreateAgent(
		context.Background(),
		protocol.CreateRequest{Name: "删除恢复助手"},
	)
	if err != nil {
		t.Fatal(err)
	}

	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath).
		ForOwner(authctx.SystemUserID)
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"ws",
		"dm",
		"crash-recovery",
		"",
	)
	transcriptID := "550e8400-e29b-41d4-a716-446655440001"
	now := time.Now().UTC()
	created, err := files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: agentValue.AgentID,
		ChannelType: "websocket", ChatType: "dm", Status: "closed",
		CreatedAt: now, LastActivity: now, Title: "crash recovery",
		SessionID: &transcriptID,
	})
	if err != nil {
		t.Fatal(err)
	}
	seedWorkspaceSessionArtifacts(
		t,
		cfg,
		agentValue.WorkspacePath,
		sessionKey,
		transcriptID,
	)
	if _, err = files.BeginSessionDeletion(
		agentValue.WorkspacePath,
		sessionKey,
		created.ConfigurationVersion,
		transcriptID,
	); err != nil {
		t.Fatal(err)
	}

	recoveryService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	recoveryService.SetRuntimeManager(runtimectx.NewManager())
	reconciled, err := recoveryService.ReconcilePendingDeletions(context.Background())
	if err != nil || reconciled != 1 {
		t.Fatalf("ReconcilePendingDeletions() reconciled=%d err=%v", reconciled, err)
	}
	if current, _, findErr := files.FindSession(
		[]string{agentValue.WorkspacePath},
		sessionKey,
	); findErr != nil || current != nil {
		t.Fatalf("recovered session still exists: item=%+v err=%v", current, findErr)
	}
	if _, statErr := os.Stat(
		sessionTranscriptFilePath(agentValue.WorkspacePath, transcriptID),
	); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered transcript still exists: %v", statErr)
	}
	pending, err := workspacestore.NewSessionFileStore(
		cfg.WorkspacePath,
	).ListPendingSessionDeletions()
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending deletion records=%d err=%v", len(pending), err)
	}
	restartedRuntime := runtimectx.NewManager()
	restartedService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	restartedService.SetRuntimeManager(restartedRuntime)
	if reconciled, err = restartedService.ReconcilePendingDeletions(
		context.Background(),
	); err != nil || reconciled != 0 {
		t.Fatalf(
			"completed tombstone restart reconcile=%d err=%v",
			reconciled,
			err,
		)
	}
	if _, err = restartedRuntime.BeginSessionDeletion(sessionKey); !errors.Is(
		err,
		runtimectx.ErrRuntimeSessionDeleted,
	) {
		t.Fatalf("completed tombstone did not restore runtime fence after restart: %v", err)
	}
	if _, err = files.UpsertSession(agentValue.WorkspacePath, *created); !errors.Is(
		err,
		workspacestore.ErrSessionDeleted,
	) {
		t.Fatalf("recovered tombstone did not block late writer: %v", err)
	}
}

func TestSessionDeletionRecoveryInvalidatesBoundAutomationTask(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	core, db := newSessionTestCoreServices(t, cfg)
	core.Session.SetRuntimeManager(runtimectx.NewManager())
	automationService := automationsvc.NewService(cfg, db, core.Agent, nil, nil, nil, nil, nil)
	core.Deletion.SetTaskCleaner(automationService)

	ctx := context.Background()
	agentValue, err := core.Agent.CreateAgent(ctx, protocol.CreateRequest{Name: "删除恢复任务助手"})
	if err != nil {
		t.Fatal(err)
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWebSocket,
		"dm",
		"crash-task-recovery",
		"",
	)
	created, err := core.Session.CreateSession(ctx, session.CreateRequest{
		SessionKey: sessionKey,
		Title:      "crash task recovery",
	})
	if err != nil {
		t.Fatal(err)
	}
	interval := 300
	task, err := automationService.CreateTask(ctx, automationdomain.CreateJobInput{
		Name:        "崩溃恢复绑定任务",
		AgentID:     agentValue.AgentID,
		Schedule:    automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &interval, Timezone: "UTC"},
		Instruction: "run",
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
			WakeMode:        automationdomain.WakeModeNow,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source:   automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	if _, err = files.BeginSessionDeletion(
		agentValue.WorkspacePath,
		sessionKey,
		created.ConfigurationVersion,
		"",
	); err != nil {
		t.Fatal(err)
	}

	reconciled, err := core.Session.ReconcilePendingDeletions(ctx)
	if err != nil || reconciled != 1 {
		t.Fatalf("ReconcilePendingDeletions() reconciled=%d err=%v", reconciled, err)
	}
	kept, err := automationService.GetTask(ctx, task.JobID)
	if err != nil || kept == nil || kept.Enabled ||
		kept.SessionBindingState != automationdomain.TaskSessionBindingStateRebindRequired {
		t.Fatalf("恢复删除后任务未停用待重绑: task=%+v err=%v", kept, err)
	}
}

func TestDeleteSessionArtifactsFencesMissingMeta(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	agentValue, err := agentService.CreateAgent(
		context.Background(),
		protocol.CreateRequest{Name: "Artifact 清理助手"},
	)
	if err != nil {
		t.Fatal(err)
	}
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	sessionService.SetRuntimeManager(runtimectx.NewManager())
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"automation",
		"dm",
		"scheduled-task:missing-meta",
		"",
	)
	if err = sessionService.DeleteSessionArtifacts(
		context.Background(),
		authctx.SystemUserID,
		agentValue.WorkspacePath,
		sessionKey,
		"",
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath).
		ForOwner(authctx.SystemUserID)
	if _, err = files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: agentValue.AgentID,
		ChannelType: "automation", ChatType: "dm", Status: "closed",
		CreatedAt: now, LastActivity: now, Title: "late",
	}); !errors.Is(err, workspacestore.ErrSessionDeleted) {
		t.Fatalf("artifact tombstone did not block missing-meta resurrection: %v", err)
	}
}
