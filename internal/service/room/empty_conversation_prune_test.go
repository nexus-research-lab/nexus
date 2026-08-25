package room_test

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	usagestore "github.com/nexus-research-lab/nexus/internal/storage/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestPruneEmptyConversationsIgnoresTitlesAndIsIdempotent(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
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

func TestPruneEmptyConversationsPreservesCanonicalGroupUserInput(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentA := createTestAgent(t, agentService, ctx, "历史判定助手A")
	agentB := createTestAgent(t, agentService, ctx, "历史判定助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "历史判定 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	usedContext, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "标题与 started 无关"},
	)
	if err != nil {
		t.Fatalf("创建有历史会话失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	emptyContext, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "仍未聊天"},
	)
	if err != nil {
		t.Fatalf("创建空白会话失败: %v", err)
	}
	setConversationCreatedAt(t, db, mainContext.Conversation.ID, time.Now().UTC().Add(-3*time.Hour))
	setConversationCreatedAt(t, db, usedContext.Conversation.ID, time.Now().UTC().Add(-2*time.Hour))
	setConversationCreatedAt(t, db, emptyContext.Conversation.ID, time.Now().UTC().Add(-time.Hour))
	seedRoomConversationLog(t, cfg.WorkspacePath, usedContext.Conversation.ID, mainContext.Room.ID)

	// Room 正文在 workspace overlay，SQL messages 可以一直是 0。
	assertSQLCount(t, db, `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, 0, usedContext.Conversation.ID)

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	usedItem := requirePruneItem(t, report, usedContext.Conversation.ID)
	if usedItem.State != "occupied" || !slices.Contains(usedItem.Reasons, "canonical_user_input_present") {
		t.Fatalf("canonical user input 必须保护会话: %+v", usedItem)
	}
	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("读取清理后上下文失败: %v", err)
	}
	if len(contexts) != 2 ||
		!hasConversationID(contexts, usedContext.Conversation.ID) ||
		!hasConversationID(contexts, emptyContext.Conversation.ID) {
		t.Fatalf("应保留有用户输入会话和最新空白页: %+v", contexts)
	}
}

func TestPruneEmptyConversationsClearsDraftWhenNoEmptyConversationRemains(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentA := createTestAgent(t, agentService, ctx, "无空白助手A")
	agentB := createTestAgent(t, agentService, ctx, "无空白助手B")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "无空白 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	seedRoomConversationLog(t, cfg.WorkspacePath, mainContext.Conversation.ID, mainContext.Room.ID)

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("修复 draft 失败: %v", err)
	}
	if report.Deleted != 0 || len(report.DraftRepairs) != 1 ||
		report.DraftRepairs[0].Action != "cleared" ||
		report.DraftRepairs[0].KeeperConversationID != "" {
		t.Fatalf("没有 confirmed-empty 时必须清除 draft: %+v", report)
	}
	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil || len(contexts) != 1 || contexts[0].Conversation.IsDraft {
		t.Fatalf("有用户输入的唯一会话不得继续是 draft: contexts=%+v err=%v", contexts, err)
	}
}

func TestPruneEmptyConversationsReadsCanonicalDMHistory(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
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
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
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

func TestPruneEmptyConversationsPreservesReferenceOnlyPersistentRecords(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentValue := createTestAgent(t, agentService, ctx, "持久引用助手")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "持久引用 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	createLegacyBlank := func(title string) *protocol.ConversationContextAggregate {
		t.Helper()
		clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
		contextValue, createErr := roomService.CreateConversation(
			ctx,
			mainContext.Room.ID,
			protocol.CreateConversationRequest{Title: title},
		)
		if createErr != nil {
			t.Fatalf("创建 %s 失败: %v", title, createErr)
		}
		return contextValue
	}
	boundReference := createLegacyBlank("automation bound 引用")
	sourceReference := createLegacyBlank("automation source 引用")
	runReference := createLegacyBlank("automation run 引用")
	usageReference := createLegacyBlank("token usage 引用")
	goalUsageReference := createLegacyBlank("Goal usage 引用")
	goalLedgerReference := createLegacyBlank("Goal ledger 引用")
	keeper := createLegacyBlank("最新空白页")
	allContexts := []*protocol.ConversationContextAggregate{
		mainContext,
		boundReference,
		sourceReference,
		runReference,
		usageReference,
		goalUsageReference,
		goalLedgerReference,
		keeper,
	}
	base := time.Now().UTC().Add(-time.Duration(len(allContexts)) * time.Hour)
	for index, contextValue := range allContexts {
		setConversationCreatedAt(
			t,
			db,
			contextValue.Conversation.ID,
			base.Add(time.Duration(index)*time.Hour),
		)
	}

	intervalSeconds := 3600
	ownerUserID := authctx.OwnerUserID(ctx)
	automationRepository := automationstore.NewRepository(cfg, db)
	if _, err = automationRepository.UpsertScheduledTask(ctx, automationdomain.ScheduledTask{
		JobID:       "reference-only-room-task",
		OwnerUserID: ownerUserID,
		Name:        "引用未聊天 Room",
		AgentID:     agentValue.AgentID,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: &intervalSeconds,
			Timezone:        "UTC",
		},
		Instruction:   "保持这个 Room 作为后续执行目标",
		ExecutionKind: automationdomain.ExecutionKindAgent,
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildRoomSharedSessionKey(boundReference.Conversation.ID),
			WakeMode:        automationdomain.WakeModeNextHeartbeat,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source: automationdomain.Source{
			Kind: automationdomain.SourceKindSystem,
			SessionKey: protocol.BuildRoomAgentSessionKey(
				sourceReference.Conversation.ID,
				agentValue.AgentID,
				protocol.RoomTypeGroup,
			),
		},
		OverlapPolicy: automationdomain.OverlapPolicySkip,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("创建 reference-only automation 失败: %v", err)
	}
	if err = automationRepository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID:       "reference-only-room-run",
		JobID:       "reference-only-room-task",
		OwnerUserID: ownerUserID,
		TriggerKind: automationdomain.TriggerKindManual,
		SessionKey:  protocol.BuildRoomSharedSessionKey(runReference.Conversation.ID),
		Status:      automationdomain.RunStatusPending,
	}); err != nil {
		t.Fatalf("创建 reference-only automation run 失败: %v", err)
	}
	if err = usagestore.NewRepository(cfg, db).Upsert(ctx, usagestore.Record{
		OwnerUserID:    ownerUserID,
		UsageKey:       "reference-only-room-usage",
		Source:         "test",
		SessionKey:     protocol.BuildRoomSharedSessionKey(usageReference.Conversation.ID),
		MessageID:      "reference-only-message",
		RoundID:        "reference-only-round",
		AgentID:        agentValue.AgentID,
		RoomID:         mainContext.Room.ID,
		ConversationID: usageReference.Conversation.ID,
		TotalTokens:    1,
		OccurredAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("创建 reference-only token usage 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO goal_usage_source_checkpoints (
    owner_user_id, runtime_session_key, source_kind, source_id,
    cumulative_actual_tokens, last_observed_at
) VALUES (?, ?, 'nxs_task', 'reference-only-source', 0, CURRENT_TIMESTAMP)`,
		ownerUserID,
		protocol.BuildRoomSharedSessionKey(goalUsageReference.Conversation.ID),
	); err != nil {
		t.Fatalf("创建 reference-only Goal usage 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO goal_usage_parent_ledger (
    owner_user_id, goal_session_key, scope_round_id, source_round_id,
    token_usage_observed, observed_at
) VALUES (?, ?, 'reference-only-scope', 'reference-only-source-round', false, CURRENT_TIMESTAMP)`,
		ownerUserID,
		protocol.BuildRoomAgentSessionKey(
			goalLedgerReference.Conversation.ID,
			agentValue.AgentID,
			protocol.RoomTypeGroup,
		),
	); err != nil {
		t.Fatalf("创建 reference-only Goal ledger 失败: %v", err)
	}

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	for _, contextValue := range allContexts[1 : len(allContexts)-1] {
		assertPruneReason(
			t,
			report,
			contextValue.Conversation.ID,
			"persistent_conversation_reference_present",
		)
	}
	if report.Deleted != 1 || report.Occupied != 6 || report.Kept != 1 {
		t.Fatalf("只能删除无引用 main，保留六类 reference-only 页与 keeper: %+v", report)
	}
	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil || len(contexts) != 7 {
		t.Fatalf("清理后应保留七个上下文: contexts=%+v err=%v", contexts, err)
	}
	for _, contextValue := range allContexts[1:] {
		if _, exists := findConversationContext(contexts, contextValue.Conversation.ID); !exists {
			t.Fatalf("reference-only/keeper 空白页不得删除: %s", contextValue.Conversation.ID)
		}
	}
}

func TestPruneEmptyConversationsSkipsUnknownArtifactEvidence(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
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
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
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

func hasConversationID(contexts []protocol.ConversationContextAggregate, conversationID string) bool {
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return true
		}
	}
	return false
}

func containsReasonPrefix(reasons []string, prefix string) bool {
	for _, reason := range reasons {
		if strings.HasPrefix(reason, prefix) {
			return true
		}
	}
	return false
}
