package room_test

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestPruneEmptyConversationsIgnoresTitlesAndIsIdempotent(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentA := createTestAgent(t, agentService, ctx, "空白清理助手A")
	agentB := createTestAgent(t, agentService, ctx, "空白清理助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "空白清理 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	olderNamed, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "显式标题也不代表聊过"},
	)
	if err != nil {
		t.Fatalf("创建旧空白会话失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	newerNamed, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "另一个显式标题"},
	)
	if err != nil {
		t.Fatalf("创建新空白会话失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	setConversationCreatedAt(t, db, mainContext.Conversation.ID, time.Now().UTC().Add(-3*time.Hour))
	setConversationCreatedAt(t, db, olderNamed.Conversation.ID, time.Now().UTC().Add(-2*time.Hour))
	setConversationCreatedAt(t, db, newerNamed.Conversation.ID, time.Now().UTC().Add(-time.Hour))

	externalSessionKey := protocol.BuildAgentSessionKey(
		agentA.AgentID,
		protocol.SessionChannelTelegramSegment,
		protocol.RoomTypeDM,
		"external-user",
		"",
	)
	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err = files.UpsertSession(agentA.WorkspacePath, protocol.Session{
		SessionKey:   externalSessionKey,
		AgentID:      agentA.AgentID,
		ChannelType:  protocol.SessionChannelTelegram,
		ChatType:     protocol.RoomTypeDM,
		Status:       "closed",
		CreatedAt:    time.Now().UTC(),
		LastActivity: time.Now().UTC(),
		Title:        "外部通道会话",
		Options:      map[string]any{},
	}); err != nil {
		t.Fatalf("创建外部通道会话失败: %v", err)
	}

	dryRun, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
	})
	if err != nil {
		t.Fatalf("dry-run 失败: %v", err)
	}
	if dryRun.Applied || dryRun.ConversationsScanned != 3 ||
		dryRun.ConfirmedEmpty != 3 || dryRun.Kept != 1 || dryRun.WouldDelete != 2 {
		t.Fatalf("dry-run 报告错误: %+v", dryRun)
	}
	if len(dryRun.DraftRepairs) != 1 ||
		dryRun.DraftRepairs[0].Action != "would_set" ||
		dryRun.DraftRepairs[0].KeeperConversationID != newerNamed.Conversation.ID {
		t.Fatalf("dry-run 必须报告 keeper draft 修复: %+v", dryRun.DraftRepairs)
	}
	if item := requirePruneItem(t, dryRun, olderNamed.Conversation.ID); item.State != "confirmed_empty" {
		t.Fatalf("显式标题不得把未聊天页判为 occupied: %+v", item)
	}
	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil || len(contexts) != 3 {
		t.Fatalf("dry-run 不得修改数据: contexts=%+v err=%v", contexts, err)
	}

	applied, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("apply 失败: %v", err)
	}
	if applied.Deleted != 2 || applied.Kept != 1 || applied.DeleteFailed != 0 {
		t.Fatalf("apply 报告错误: %+v", applied)
	}
	if len(applied.DraftRepairs) != 1 ||
		applied.DraftRepairs[0].Action != "set" ||
		applied.DraftRepairs[0].KeeperConversationID != newerNamed.Conversation.ID {
		t.Fatalf("apply 必须把 keeper 设为唯一 draft: %+v", applied.DraftRepairs)
	}
	contexts, err = roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil || len(contexts) != 1 {
		t.Fatalf("apply 后应只保留一个空白页: contexts=%+v err=%v", contexts, err)
	}
	if contexts[0].Conversation.ID != newerNamed.Conversation.ID {
		t.Fatalf("应保留最新空白页: got=%s want=%s", contexts[0].Conversation.ID, newerNamed.Conversation.ID)
	}
	if exists, err := files.SessionArtifactsExist(agentA.WorkspacePath, externalSessionKey); err != nil || !exists {
		t.Fatalf("外部通道 session 不得受影响: exists=%v err=%v", exists, err)
	}
	ensured, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{},
	)
	if err != nil {
		t.Fatalf("清理后确保 draft 失败: %v", err)
	}
	if ensured.Conversation.ID != newerNamed.Conversation.ID {
		t.Fatalf(
			"清理后再次点击 + 必须复用 keeper: got=%s want=%s",
			ensured.Conversation.ID,
			newerNamed.Conversation.ID,
		)
	}
	contexts, err = roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil || len(contexts) != 1 {
		t.Fatalf("再次确保 draft 不得新增 conversation: contexts=%+v err=%v", contexts, err)
	}

	repeated, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("重复 apply 失败: %v", err)
	}
	if repeated.Deleted != 0 || repeated.Kept != 1 || repeated.WouldDelete != 0 {
		t.Fatalf("重复 apply 必须幂等: %+v", repeated)
	}
}

func TestPruneEmptyConversationsReadsCanonicalDMHistory(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentValue := createTestAgent(t, agentService, ctx, "DM 历史助手")
	mainContext, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建 DM room 失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	emptyContext, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "DM 空白页"},
	)
	if err != nil {
		t.Fatalf("创建 DM 空白会话失败: %v", err)
	}
	setConversationCreatedAt(t, db, mainContext.Conversation.ID, time.Now().UTC().Add(-2*time.Hour))
	setConversationCreatedAt(t, db, emptyContext.Conversation.ID, time.Now().UTC().Add(-time.Hour))

	sessionKey := protocol.BuildRoomAgentSessionKey(
		mainContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeDM,
	)
	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	if err = history.AppendRoundMarker(
		agentValue.WorkspacePath,
		sessionKey,
		"round-user-input",
		"真实用户输入",
		time.Now().UnixMilli(),
	); err != nil {
		t.Fatalf("写入 DM canonical history 失败: %v", err)
	}

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("清理 DM room 失败: %v", err)
	}
	mainItem := requirePruneItem(t, report, mainContext.Conversation.ID)
	if mainItem.State != "occupied" || !slices.Contains(mainItem.Reasons, "canonical_user_input_present") {
		t.Fatalf("DM canonical history 必须保护会话: %+v", mainItem)
	}
	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil || len(contexts) != 2 {
		t.Fatalf("有用户输入 DM + 一个空白页都应保留: contexts=%+v err=%v", contexts, err)
	}
}

func TestPruneEmptyConversationsPreservesConservativeActivityEvidence(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentA := createTestAgent(t, agentService, ctx, "安全证据助手A")
	agentB := createTestAgent(t, agentService, ctx, "安全证据助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "安全证据 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	createNamed := func(title string) *protocol.ConversationContextAggregate {
		t.Helper()
		clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
		item, createErr := roomService.CreateConversation(
			ctx,
			mainContext.Room.ID,
			protocol.CreateConversationRequest{Title: title},
		)
		if createErr != nil {
			t.Fatalf("创建 %s 失败: %v", title, createErr)
		}
		return item
	}
	databaseMessage := createNamed("数据库消息证据")
	sdkSession := createNamed("SDK session 证据")
	sharedArtifact := createNamed("共享目录证据")
	privateArtifact := createNamed("成员目录证据")
	goalEvidence := createNamed("Goal 证据")
	emptyKeeper := createNamed("最新空白页")

	base := time.Now().UTC().Add(-8 * time.Hour)
	for index, conversationID := range []string{
		mainContext.Conversation.ID,
		databaseMessage.Conversation.ID,
		sdkSession.Conversation.ID,
		sharedArtifact.Conversation.ID,
		privateArtifact.Conversation.ID,
		goalEvidence.Conversation.ID,
		emptyKeeper.Conversation.ID,
	} {
		setConversationCreatedAt(t, db, conversationID, base.Add(time.Duration(index)*time.Hour))
	}

	databaseSessionID := findRoomSessionID(t, *databaseMessage, agentA.AgentID)
	seedRoomDatabaseMessageRound(t, db, databaseMessage.Conversation.ID, databaseSessionID, "prune-evidence")
	if err = roomService.UpdateSessionRuntimeIdentity(
		ctx,
		findRoomSessionID(t, *sdkSession, agentA.AgentID),
		"sdk-session-prune-evidence",
		"",
	); err != nil {
		t.Fatalf("写入 SDK session 证据失败: %v", err)
	}
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	if err = roomHistory.AppendInlineMessage(
		sharedArtifact.Room.OwnerUserID,
		sharedArtifact.Conversation.ID,
		protocol.Message{
			"message_id":      "result-artifact-only",
			"session_key":     protocol.BuildRoomSharedSessionKey(sharedArtifact.Conversation.ID),
			"room_id":         mainContext.Room.ID,
			"conversation_id": sharedArtifact.Conversation.ID,
			"role":            "result",
			"content":         "没有 user message 的保守证据",
			"timestamp":       time.Now().UnixMilli(),
		}); err != nil {
		t.Fatalf("写入共享目录证据失败: %v", err)
	}
	seedRoomPrivateSession(
		t,
		workspacestore.NewSessionFileStore(cfg.WorkspacePath),
		agentA.WorkspacePath,
		protocol.RoomTypeGroup,
		privateArtifact.Conversation.ID,
		agentA.AgentID,
	)
	roomService.SetGoalCleaner(&fakePruneGoalCleaner{
		conversationIDs: map[string]struct{}{goalEvidence.Conversation.ID: {}},
	})

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	assertPruneReason(t, report, databaseMessage.Conversation.ID, "database_messages_present")
	assertPruneReason(t, report, sdkSession.Conversation.ID, "sdk_session_present:"+agentA.AgentID)
	assertPruneReason(t, report, sharedArtifact.Conversation.ID, "room_conversation_artifacts_present")
	assertPruneReason(t, report, privateArtifact.Conversation.ID, "agent_session_artifacts_present:"+agentA.AgentID)
	assertPruneReason(t, report, goalEvidence.Conversation.ID, "goal_present")
	if report.Deleted != 1 || report.Kept != 1 || report.Occupied != 5 {
		t.Fatalf("只能删除旧空白 main 并保留五类证据与最新空白页: %+v", report)
	}
	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil || len(contexts) != 6 {
		t.Fatalf("清理后上下文数量错误: contexts=%+v err=%v", contexts, err)
	}
}

func TestPruneEmptyConversationsSkipsUnknownArtifactEvidence(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentA := createTestAgent(t, agentService, ctx, "未知证据助手A")
	agentB := createTestAgent(t, agentService, ctx, "未知证据助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "未知证据 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	if _, err = roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "不能确认的空白页"},
	); err != nil {
		t.Fatalf("创建空白页失败: %v", err)
	}
	if _, err = db.Exec(`UPDATE agents SET workspace_path = ? WHERE id = ?`, "/outside/nexus-maintenance-root", agentA.AgentID); err != nil {
		t.Fatalf("制造越界 workspace 证据失败: %v", err)
	}

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("unknown 应跳过而不是让命令失败: %v", err)
	}
	if report.Deleted != 0 || report.Unknown != 2 {
		t.Fatalf("无法读取任一证据时不得删除: %+v", report)
	}
	for _, item := range report.Items {
		if item.Action != "skip_unknown" ||
			!containsReasonPrefix(item.Reasons, "agent_session_artifact_probe_failed:"+agentA.AgentID+":") {
			t.Fatalf("unknown item 缺少证据错误: %+v", item)
		}
	}
}

func TestPruneEmptyConversationsSkipsUnknownPersistentReferenceProbe(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentValue := createTestAgent(t, agentService, ctx, "引用探测失败助手")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "引用探测失败 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	if _, err = roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "无法确认引用的空白页"},
	); err != nil {
		t.Fatalf("创建空白页失败: %v", err)
	}
	if _, err = db.Exec(`DROP TABLE goal_usage_source_checkpoints`); err != nil {
		t.Fatalf("制造持久引用查询错误失败: %v", err)
	}

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("引用探测错误应降级为 unknown/skip: %v", err)
	}
	if report.Deleted != 0 || report.Unknown != 2 {
		t.Fatalf("持久引用无法确认时不得删除: %+v", report)
	}
	for _, item := range report.Items {
		if item.Action != "skip_unknown" ||
			!containsReasonPrefix(item.Reasons, "persistent_reference_probe_failed:") {
			t.Fatalf("unknown item 缺少持久引用错误: %+v", item)
		}
	}
}

type fakePruneGoalCleaner struct {
	conversationIDs map[string]struct{}
}

func (f *fakePruneGoalCleaner) HasGoalForRoomConversation(_ context.Context, conversationID string) (bool, error) {
	_, ok := f.conversationIDs[conversationID]
	return ok, nil
}

func (f *fakePruneGoalCleaner) DeleteGoalsForRoomConversations(
	_ context.Context,
	conversationIDs []string,
) (int, error) {
	return len(conversationIDs), nil
}

func (f *fakePruneGoalCleaner) DeleteGoalsForRoomMember(
	_ context.Context,
	_ string,
	conversationIDs []string,
) (int, error) {
	return len(conversationIDs), nil
}

func clearRoomDraftForLegacyFixture(t *testing.T, db *sql.DB, roomID string) {
	t.Helper()
	if _, err := db.Exec(
		`UPDATE conversations SET is_draft = false WHERE room_id = ? AND is_draft = true`,
		roomID,
	); err != nil {
		t.Fatalf("模拟旧版本已遗留的非 draft 空白页失败: %v", err)
	}
}

func setConversationCreatedAt(t *testing.T, db *sql.DB, conversationID string, createdAt time.Time) {
	t.Helper()
	if _, err := db.Exec(
		`UPDATE conversations SET created_at = ?, updated_at = ?, last_activity_at = ? WHERE id = ?`,
		createdAt,
		createdAt,
		createdAt,
		conversationID,
	); err != nil {
		t.Fatalf("设置 conversation 时间失败: %v", err)
	}
}

func requirePruneItem(
	t *testing.T,
	report roomsvc.EmptyConversationPruneReport,
	conversationID string,
) roomsvc.EmptyConversationPruneItem {
	t.Helper()
	for _, item := range report.Items {
		if item.ConversationID == conversationID {
			return item
		}
	}
	t.Fatalf("未找到 prune item: conversation=%s report=%+v", conversationID, report)
	return roomsvc.EmptyConversationPruneItem{}
}

func assertPruneReason(
	t *testing.T,
	report roomsvc.EmptyConversationPruneReport,
	conversationID string,
	reason string,
) {
	t.Helper()
	item := requirePruneItem(t, report, conversationID)
	if item.State != "occupied" || !slices.Contains(item.Reasons, reason) {
		t.Fatalf("conversation %s 缺少保护证据 %s: %+v", conversationID, reason, item)
	}
}

func containsReasonPrefix(reasons []string, prefix string) bool {
	for _, reason := range reasons {
		if strings.HasPrefix(reason, prefix) {
			return true
		}
	}
	return false
}
