package dm

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

func TestCreateTransientSessionIsHiddenAndDoesNotForkTranscript(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	agents := newDMAgentService(t, cfg)
	if err := agents.EnsureReady(context.Background()); err != nil {
		t.Fatalf("初始化 Agent 失败: %v", err)
	}
	service := NewService(
		cfg,
		agents,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)
	sessionKey := protocol.BuildAgentSessionKey(
		cfg.DefaultAgentID,
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"preview-a",
		"",
	)
	created, err := service.CreateTransientSession(context.Background(), TransientSessionRequest{
		AgentID:          cfg.DefaultAgentID,
		TargetSessionKey: sessionKey,
		Purpose:          protocol.SessionPurposeWorkGraphDistillation,
		Title:            "保存工作图草图",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.SessionKey != sessionKey || !protocol.SessionIsHiddenFromDirectory(*created) ||
		protocol.SessionPurpose(*created) != protocol.SessionPurposeWorkGraphDistillation {
		t.Fatalf("transient Session = %#v", created)
	}
	if created.SessionID != nil ||
		created.Options[protocol.OptionRuntimeForkSourceSessionID] != nil ||
		created.Options[protocol.OptionRuntimeForkMessageID] != nil {
		t.Fatalf("isolated Session inherited a transcript: %#v", created)
	}

	_, err = service.CreateTransientSession(context.Background(), TransientSessionRequest{
		AgentID: cfg.DefaultAgentID,
		TargetSessionKey: protocol.BuildAgentSessionKey(
			cfg.DefaultAgentID,
			protocol.SessionChannelWebSocketSegment,
			protocol.RoomTypeDM,
			"preview-a",
			"",
		),
		Purpose: protocol.SessionPurposeWorkGraphDistillation,
	})
	if err == nil {
		t.Fatal("transient internal Session accepted a user WebSocket key")
	}
}

func TestCreateTransientWorkGraphEditorSessionUsesHiddenWebSocketDMWithoutFork(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	agents := newDMAgentService(t, cfg)
	if err := agents.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewService(
		cfg,
		agents,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)
	sessionKey := protocol.BuildAgentSessionKey(
		cfg.DefaultAgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"editor-a",
		"",
	)
	created, err := service.CreateTransientSession(context.Background(), TransientSessionRequest{
		AgentID: cfg.DefaultAgentID, TargetSessionKey: sessionKey,
		Purpose: protocol.SessionPurposeWorkGraphEditor, Title: "调整草图",
		DisplayAfterUnixMilli: 12345,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !protocol.SessionIsHiddenFromDirectory(*created) ||
		protocol.SessionPurpose(*created) != protocol.SessionPurposeWorkGraphEditor ||
		fmt.Sprint(created.Options[protocol.OptionSessionDisplayAfterUnixMilli]) != "12345" ||
		created.SessionID != nil || created.Options[protocol.OptionRuntimeForkSourceSessionID] != nil {
		t.Fatalf("hidden WorkGraph editor Session = %#v", created)
	}
}
