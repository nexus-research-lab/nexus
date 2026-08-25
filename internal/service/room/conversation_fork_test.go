package room_test

import (
	"context"
	"errors"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type fakeConversationSessionForker struct {
	sourceSessionKey string
	targetSessionKey string
	targetRoundID    string
	sourceSessionID  string
	sourceMessageID  string
	forkErr          error
	onFork           func()
}

func (f *fakeConversationSessionForker) PrepareConversationFork(
	_ context.Context,
	sourceSessionKey string,
	targetRoundID string,
) (string, string, error) {
	f.sourceSessionKey = sourceSessionKey
	f.targetRoundID = targetRoundID
	return "source-sdk-session", "source-boundary-message", nil
}

func (f *fakeConversationSessionForker) ForkConversationSession(
	_ context.Context,
	sourceSessionKey string,
	targetSessionKey string,
	targetRoundID string,
	sourceSessionID string,
	sourceMessageID string,
) error {
	f.sourceSessionKey = sourceSessionKey
	f.targetSessionKey = targetSessionKey
	f.targetRoundID = targetRoundID
	f.sourceSessionID = sourceSessionID
	f.sourceMessageID = sourceMessageID
	if f.onFork != nil {
		f.onFork()
	}
	return f.forkErr
}

func TestRoomServiceForkFailureCompensatesAfterRequestCancellation(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	baseCtx := context.Background()
	if err = agentService.EnsureReady(baseCtx); err != nil {
		t.Fatalf("初始化主智能体失败: %v", err)
	}
	source, err := roomService.EnsureDirectRoom(baseCtx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("创建 source DM 失败: %v", err)
	}
	requestCtx, cancel := context.WithCancel(baseCtx)
	forker := &fakeConversationSessionForker{
		forkErr: context.Canceled,
		onFork:  cancel,
	}
	roomService.SetConversationSessionForker(forker)
	artifactErr := errors.New("cleanup fork artifacts")
	roomService.SetSessionArtifactDeletionCoordinator(&fakeRoomSessionArtifactDeletionCoordinator{
		err: artifactErr,
	})

	if _, err = roomService.ForkConversation(
		requestCtx,
		source.Room.ID,
		source.Conversation.ID,
		protocol.ForkConversationRequest{RoundID: "round-finished"},
	); !errors.Is(err, context.Canceled) || !errors.Is(err, artifactErr) {
		t.Fatalf("fork 应保留原始错误和提交后清理错误: %v", err)
	}
	assertSQLCount(t, db, "SELECT COUNT(*) FROM conversations WHERE room_id = ?", 1, source.Room.ID)
}

func TestRoomServiceForkConversationCreatesIndependentDMConversation(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	forker := &fakeConversationSessionForker{}
	roomService.SetConversationSessionForker(forker)

	ctx := context.Background()
	if err = agentService.EnsureReady(ctx); err != nil {
		t.Fatalf("初始化主智能体失败: %v", err)
	}
	source, err := roomService.EnsureDirectRoom(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("创建 source DM 失败: %v", err)
	}
	source, err = roomService.UpdateConversation(
		ctx,
		source.Room.ID,
		source.Conversation.ID,
		protocol.UpdateConversationRequest{Title: "原对话"},
	)
	if err != nil {
		t.Fatalf("更新 source 标题失败: %v", err)
	}
	if err = roomService.UpdateSessionRuntimeIdentity(
		ctx,
		source.Sessions[0].ID,
		"source-sdk-session",
		"",
	); err != nil {
		t.Fatalf("写入 source SDK session 失败: %v", err)
	}
	if _, err = db.ExecContext(ctx, `
UPDATE sessions
SET options_json = '{"session_provider":"provider-a","session_model":"model-a","runtime_kind":"nxs"}'
WHERE conversation_id = ?`, source.Conversation.ID); err != nil {
		t.Fatalf("写入 source session options 失败: %v", err)
	}

	target, err := roomService.ForkConversation(
		ctx,
		source.Room.ID,
		source.Conversation.ID,
		protocol.ForkConversationRequest{RoundID: "round-finished"},
	)
	if err != nil {
		t.Fatalf("创建 conversation fork 失败: %v", err)
	}
	if target.Conversation.ID == source.Conversation.ID || target.Conversation.IsDraft {
		t.Fatalf("fork conversation 身份不正确: %+v", target.Conversation)
	}
	if target.Conversation.Title != "原对话" || len(target.Sessions) != 1 {
		t.Fatalf("fork conversation 投影不正确: %+v", target)
	}
	options := target.Sessions[0].Options
	if options[protocol.OptionSessionProvider] != "provider-a" ||
		options[protocol.OptionSessionModel] != "model-a" {
		t.Fatalf("fork session 未继承显式模型设置: %+v", options)
	}
	if _, exists := options[protocol.OptionRuntimeKind]; exists {
		t.Fatalf("fork session 不应继承 source runtime 指纹: %+v", options)
	}
	wantSourceKey := protocol.BuildRoomAgentSessionKey(
		source.Conversation.ID,
		cfg.DefaultAgentID,
		protocol.RoomTypeDM,
	)
	wantTargetKey := protocol.BuildRoomAgentSessionKey(
		target.Conversation.ID,
		cfg.DefaultAgentID,
		protocol.RoomTypeDM,
	)
	if forker.sourceSessionKey != wantSourceKey ||
		forker.targetSessionKey != wantTargetKey ||
		forker.targetRoundID != "round-finished" ||
		forker.sourceSessionID != "source-sdk-session" ||
		forker.sourceMessageID != "source-boundary-message" {
		t.Fatalf("fork callback 参数不正确: %+v", forker)
	}
	if options[protocol.OptionRuntimeForkSourceSessionID] != "source-sdk-session" ||
		options[protocol.OptionRuntimeForkMessageID] != "source-boundary-message" {
		t.Fatalf("fork 依赖未与目标 Session 原子持久化: %+v", options)
	}

	artifacts := &fakeRoomSessionArtifactDeletionCoordinator{}
	roomService.SetSessionArtifactDeletionCoordinator(artifacts)
	if _, err = roomService.DeleteConversation(
		ctx,
		source.Room.ID,
		source.Conversation.ID,
	); err != nil {
		t.Fatalf("删除 fork source 失败: %v", err)
	}
	calls := artifacts.Calls()
	if len(calls) != 1 || calls[0].cleanupSessionID != "" {
		t.Fatalf("尚未物化的 fork 应保护 source transcript: %+v", calls)
	}
	remaining, err := roomService.GetConversationContext(ctx, target.Conversation.ID)
	if err != nil || remaining == nil {
		t.Fatalf("删除 source 后 target fork 不可读: target=%+v err=%v", remaining, err)
	}
}
