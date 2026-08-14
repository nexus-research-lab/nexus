package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	titlegensvc "github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestSessionServiceLifecycle(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "测试会话助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "launcher-app-"+agentA.AgentID, "")
	runtimeManager.MarkSubagentHistory(dmKey)
	created, err := sessionService.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: dmKey,
		Title:      "Launcher App",
	})
	if err != nil {
		t.Fatalf("创建普通 session 失败: %v", err)
	}
	if created.Title != "Launcher App" {
		t.Fatalf("session 标题不正确: got=%s", created.Title)
	}

	dmSessionID := bindTranscriptSessionID(t, cfg, agentA.WorkspacePath, created)
	seedWorkspaceSessionArtifacts(t, cfg, agentA.WorkspacePath, dmKey, dmSessionID)
	previousDMSessionID := "650e8400-e29b-41d4-a716-446655440000"
	created.TranscriptSessionIDs = []string{previousDMSessionID}
	if _, err = workspacestore.NewSessionFileStore(cfg.WorkspacePath).UpsertSession(
		agentA.WorkspacePath,
		*created,
	); err != nil {
		t.Fatalf("回写 transcript lineage 失败: %v", err)
	}
	writeSessionTranscriptFixture(t, agentA.WorkspacePath, previousDMSessionID, []map[string]any{{
		"type":      "assistant",
		"uuid":      "previous-transcript",
		"sessionId": previousDMSessionID,
	}})

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}
	seedRoomConversationMessages(t, cfg, dmContext.Conversation.ID)
	dmSessionKey := protocol.BuildRoomAgentSessionKey(
		dmContext.Conversation.ID,
		agentA.AgentID,
		protocol.RoomTypeDM,
	)
	dmSessionStore := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	roomBacked, _, err := dmSessionStore.FindSession(
		[]string{agentA.WorkspacePath},
		dmSessionKey,
	)
	if err != nil {
		t.Fatalf("读取 Room-backed workspace session 失败: %v", err)
	}
	if roomBacked == nil {
		roomBacked = &protocol.Session{
			SessionKey:     dmSessionKey,
			AgentID:        agentA.AgentID,
			ConversationID: &dmContext.Conversation.ID,
			CreatedAt:      time.Now().UTC(),
			Options:        map[string]any{},
		}
	}
	roomBacked.MessageCount = 11
	roomBacked.LastActivity = time.Now().UTC().Add(time.Minute)
	if _, err = dmSessionStore.UpsertSession(agentA.WorkspacePath, *roomBacked); err != nil {
		t.Fatalf("写入 Room-backed workspace 进度失败: %v", err)
	}

	sessions, err := sessionService.ListSessions(ctx)
	if err != nil {
		t.Fatalf("列出 sessions 失败: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatalf("session 列表未合并 room 视图: got=%d", len(sessions))
	}
	listedRoomSession := findSessionByKey(sessions, dmSessionKey)
	if listedRoomSession == nil || listedRoomSession.MessageCount != 11 {
		t.Fatalf("全部 session 列表未保留 workspace 消息进度: %+v", listedRoomSession)
	}

	agentSessions, err := sessionService.ListAgentSessions(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取 agent sessions 失败: %v", err)
	}
	if len(agentSessions) < 2 {
		t.Fatalf("agent sessions 数量不正确: got=%d", len(agentSessions))
	}
	listedRoomSession = findSessionByKey(agentSessions, dmSessionKey)
	if listedRoomSession == nil || listedRoomSession.MessageCount != 11 {
		t.Fatalf("Agent session 列表未保留 workspace 消息进度: %+v", listedRoomSession)
	}
	loadedRoomSession, err := sessionService.GetSession(ctx, dmSessionKey)
	if err != nil || loadedRoomSession.MessageCount != 11 {
		t.Fatalf("单 session 读取未保留 workspace 消息进度: item=%+v err=%v", loadedRoomSession, err)
	}

	messages, err := sessionService.GetSessionMessages(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取普通 session 消息失败: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("消息归一化结果不正确: got=%d want=3 messages=%+v", len(messages), messages)
	}
	contentBlocks, ok := messages[1]["content"].([]map[string]any)
	if !ok && messages[1]["content"] != nil {
		rawBlocks, okAny := messages[1]["content"].([]any)
		if okAny {
			contentBlocks = make([]map[string]any, 0, len(rawBlocks))
			for _, item := range rawBlocks {
				if payload, okMap := item.(map[string]any); okMap {
					contentBlocks = append(contentBlocks, payload)
				}
			}
			ok = true
		}
	}
	if !ok || len(contentBlocks) != 1 || contentBlocks[0]["type"] != "text" || contentBlocks[0]["text"] != "最终回复" {
		t.Fatalf("消息压缩未保留最新快照: %+v", messages[1])
	}
	if _, exists := messages[1]["stream_status"]; exists {
		t.Fatalf("未终止 round 的 assistant 不应补写 stream_status: %+v", messages[1])
	}
	if messages[2]["role"] != "assistant" {
		t.Fatalf("未终止 round 应追加 synthetic assistant: %+v", messages)
	}
	if strings.TrimSpace(stringValue(messages[2]["stop_reason"])) != "cancelled" {
		t.Fatalf("synthetic assistant stop_reason 不正确: %+v", messages[2])
	}
	summary, ok := messages[2]["result_summary"].(map[string]any)
	if !ok || strings.TrimSpace(stringValue(summary["subtype"])) != "interrupted" {
		t.Fatalf("未终止 round 应把 interrupted 摘要挂到 synthetic assistant 上: %+v", messages[2])
	}

	messagePage, err := sessionService.GetSessionMessagesPage(ctx, dmKey, sessionsvc.MessagePageRequest{
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("分页读取普通 session 消息失败: %v", err)
	}
	if len(messagePage.Items) != 3 || messagePage.HasMore {
		t.Fatalf("普通 session 最新页结果不正确: %+v", messagePage)
	}
	if messagePage.Items[0]["message_id"] != "msg_user_round_1" {
		t.Fatalf("普通 session 最新页起点不正确: %+v", messagePage.Items)
	}
	if messagePage.Items[1]["message_id"] != "msg_assistant_1" {
		t.Fatalf("普通 session 最新页终点不正确: %+v", messagePage.Items)
	}
	if messagePage.Items[2]["message_id"] != "assistant_interrupt_round_1" {
		t.Fatalf("普通 session synthetic assistant 不正确: %+v", messagePage.Items)
	}

	roomMessages, err := sessionService.GetSessionMessages(ctx, protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID))
	if err != nil {
		t.Fatalf("读取 Room 共享流失败: %v", err)
	}
	if len(roomMessages) != 2 {
		t.Fatalf("Room 共享消息数量不正确: got=%d want=2 messages=%+v", len(roomMessages), roomMessages)
	}
	if _, exists := roomMessages[0]["stream_status"]; exists {
		t.Fatalf("Room assistant 历史回放不应补写 stream_status: %+v", roomMessages[0])
	}
	roomSummary, ok := roomMessages[1]["result_summary"].(map[string]any)
	if !ok || strings.TrimSpace(stringValue(roomSummary["subtype"])) != "interrupted" {
		t.Fatalf("Room 未终止 round 应把 interrupted 摘要挂到 synthetic assistant 上: %+v", roomMessages)
	}

	roomMessagePage, err := sessionService.GetSessionMessagesPage(
		ctx,
		protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID),
		sessionsvc.MessagePageRequest{Limit: 1},
	)
	if err != nil {
		t.Fatalf("分页读取 Room 共享流失败: %v", err)
	}
	if len(roomMessagePage.Items) != 2 || roomMessagePage.HasMore {
		t.Fatalf("Room 最新页结果不正确: %+v", roomMessagePage)
	}
	if roomMessagePage.Items[0]["role"] != "assistant" {
		t.Fatalf("Room 最新页应返回 assistant 聚合结果: %+v", roomMessagePage.Items)
	}
	if roomMessagePage.Items[1]["role"] != "assistant" {
		t.Fatalf("Room synthetic assistant 应保留在同一轮分页结果里: %+v", roomMessagePage.Items)
	}

	updatedTitle := "Launcher 重命名"
	updated, err := sessionService.UpdateSession(ctx, dmKey, sessionsvc.UpdateRequest{Title: &updatedTitle})
	if err != nil {
		t.Fatalf("更新 session 失败: %v", err)
	}
	if updated.Title != updatedTitle {
		t.Fatalf("更新标题失败: got=%s want=%s", updated.Title, updatedTitle)
	}
	if _, err = db.Exec(`
INSERT INTO automation_delivery_routes (
    route_id, agent_id, session_key, mode, enabled
) VALUES ('session-route', ?, ?, 'last', TRUE)`, agentA.AgentID, dmKey); err != nil {
		t.Fatalf("准备 Session route 失败: %v", err)
	}

	if err = sessionService.DeleteSession(ctx, dmKey); err != nil {
		t.Fatalf("删除 session 失败: %v", err)
	}
	if _, err = sessionService.GetSession(ctx, dmKey); err == nil {
		t.Fatal("删除后不应还能读取到 session")
	}
	if _, err = os.Stat(sessionTranscriptFilePath(agentA.WorkspacePath, dmSessionID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除 session 后 transcript 仍残留: %v", err)
	}
	if _, err = os.Stat(sessionTranscriptFilePath(agentA.WorkspacePath, previousDMSessionID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除 session 后历史 transcript 仍残留: %v", err)
	}
	if runtimeManager.HasSubagentHistory(dmKey) {
		t.Fatal("删除 session 后 runtime state 仍残留")
	}
	var routeCount int
	if err = db.QueryRow(
		"SELECT COUNT(*) FROM automation_delivery_routes WHERE session_key = ?",
		dmKey,
	).Scan(&routeCount); err != nil {
		t.Fatalf("读取 Session route 失败: %v", err)
	}
	if routeCount != 0 {
		t.Fatalf("删除 Session 后仍残留 delivery route: %d", routeCount)
	}
}

func TestSessionRuntimeSettingsPersistWithoutChangingAgentDefaults(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "Session 设置助手",
		Options: &protocol.Options{
			Provider:       "agent-provider",
			Model:          "agent-model",
			PermissionMode: "default",
		},
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"ws",
		"dm",
		"session-settings",
		"",
	)
	if _, err = sessionService.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: dmKey,
	}); err != nil {
		t.Fatalf("创建 DM Session 失败: %v", err)
	}
	want := protocol.SessionRuntimeSettings{
		Provider:       "session-provider",
		Model:          "session-model",
		PermissionMode: "acceptEdits",
	}
	if _, err = sessionService.UpdateRuntimeSettings(ctx, dmKey, want); err != nil {
		t.Fatalf("更新 DM Session 设置失败: %v", err)
	}
	if got, getErr := sessionService.GetRuntimeSettings(ctx, dmKey); getErr != nil {
		t.Fatalf("读取 DM Session 设置失败: %v", getErr)
	} else if got != want {
		t.Fatalf("DM Session 设置 = %+v, want %+v", got, want)
	}

	roomContext, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建 Room Session 失败: %v", err)
	}
	roomKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeDM,
	)
	if _, err = sessionService.UpdateRuntimeSettings(ctx, roomKey, want); err != nil {
		t.Fatalf("更新 Room Session 设置失败: %v", err)
	}
	if got, getErr := sessionService.GetRuntimeSettings(ctx, roomKey); getErr != nil {
		t.Fatalf("读取 Room Session 设置失败: %v", getErr)
	} else if got != want {
		t.Fatalf("Room Session 设置 = %+v, want %+v", got, want)
	}

	agentPeer, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "Session 设置协作者",
		Options: &protocol.Options{
			Provider:       "peer-provider",
			Model:          "peer-default-model",
			PermissionMode: "plan",
		},
	})
	if err != nil {
		t.Fatalf("创建协作 Agent 失败: %v", err)
	}
	groupContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID, agentPeer.AgentID},
		Name:     "Session 设置群聊",
	})
	if err != nil {
		t.Fatalf("创建群聊 Session 失败: %v", err)
	}
	primaryRoomKey := protocol.BuildRoomAgentSessionKey(
		groupContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeGroup,
	)
	peerRoomKey := protocol.BuildRoomAgentSessionKey(
		groupContext.Conversation.ID,
		agentPeer.AgentID,
		protocol.RoomTypeGroup,
	)
	peerSettings := protocol.SessionRuntimeSettings{
		Provider:       "peer-session-provider",
		Model:          "peer-session-model",
		PermissionMode: "plan",
	}
	if _, err = sessionService.UpdateRuntimeSettings(
		ctx,
		peerRoomKey,
		peerSettings,
	); err != nil {
		t.Fatalf("更新协作 Agent Session 设置失败: %v", err)
	}
	if _, err = sessionService.UpdateRuntimeSettings(
		ctx,
		primaryRoomKey,
		want,
	); err != nil {
		t.Fatalf("更新群聊统一权限失败: %v", err)
	}
	gotPeer, err := sessionService.GetRuntimeSettings(ctx, peerRoomKey)
	if err != nil {
		t.Fatalf("读取协作 Agent Session 设置失败: %v", err)
	}
	if gotPeer.Provider != peerSettings.Provider ||
		gotPeer.Model != peerSettings.Model ||
		gotPeer.PermissionMode != want.PermissionMode {
		t.Fatalf(
			"群聊权限应统一且模型应保持独立: got=%+v peer=%+v room=%+v",
			gotPeer,
			peerSettings,
			want,
		)
	}
	resetRoomPermission := want
	resetRoomPermission.PermissionMode = ""
	if _, err = sessionService.UpdateRuntimeSettings(
		ctx,
		primaryRoomKey,
		resetRoomPermission,
	); err != nil {
		t.Fatalf("重置群聊统一权限失败: %v", err)
	}
	gotPeer, err = sessionService.GetRuntimeSettings(ctx, peerRoomKey)
	if err != nil {
		t.Fatalf("重置后读取协作 Agent Session 设置失败: %v", err)
	}
	if gotPeer.Provider != peerSettings.Provider ||
		gotPeer.Model != peerSettings.Model ||
		gotPeer.PermissionMode != "" {
		t.Fatalf("群聊权限重置应统一清空且保持模型: %+v", gotPeer)
	}

	unchangedAgent, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取 Agent 失败: %v", err)
	}
	if unchangedAgent.Options.Provider != "agent-provider" ||
		unchangedAgent.Options.Model != "agent-model" ||
		unchangedAgent.Options.PermissionMode != "default" {
		t.Fatalf("Session 设置不应修改 Agent 默认值: %+v", unchangedAgent.Options)
	}
}

func TestSessionLocalDirectoriesRequireDesktopAndPersist(t *testing.T) {
	cfg := newSessionTestConfig(t)
	cfg.Debug = true
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "本机目录助手",
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"ws",
		"dm",
		"local-directories",
		"",
	)
	if _, err = sessionService.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: sessionKey,
	}); err != nil {
		t.Fatalf("创建 Session 失败: %v", err)
	}
	if _, err = sessionService.GetLocalDirectories(ctx, sessionKey); !errors.Is(
		err,
		sessionsvc.ErrLocalDirectoriesUnavailable,
	) {
		t.Fatalf("普通 Web 读取错误 = %v", err)
	}

	desktopCtx := authctx.WithInteractiveHumanEvidence(
		ctx,
		"desktop_session_token",
	)
	directory := t.TempDir()
	secondDirectory := t.TempDir()
	updated, err := sessionService.UpdateLocalDirectories(
		desktopCtx,
		sessionKey,
		protocol.SessionLocalDirectories{
			Directories: []string{directory, directory},
		},
	)
	if err != nil {
		t.Fatalf("更新本机目录失败: %v", err)
	}
	if len(updated.Directories) != 1 || updated.Directories[0] != directory {
		t.Fatalf("本机目录未去重: %+v", updated)
	}
	loaded, err := sessionService.GetLocalDirectories(desktopCtx, sessionKey)
	if err != nil || len(loaded.Directories) != 1 || loaded.Directories[0] != directory {
		t.Fatalf("读取持久化本机目录 = %+v, err=%v", loaded, err)
	}
	if _, err = sessionService.UpdateLocalDirectories(
		desktopCtx,
		sessionKey,
		protocol.SessionLocalDirectories{Directories: []string{"relative"}},
	); !errors.Is(err, sessionsvc.ErrInvalidLocalDirectories) {
		t.Fatalf("相对目录错误 = %v", err)
	}

	directRoom, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建 Room-backed DM 失败: %v", err)
	}
	roomSessionKey := protocol.BuildRoomAgentSessionKey(
		directRoom.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeDM,
	)
	if _, err = sessionService.UpdateLocalDirectories(
		desktopCtx,
		roomSessionKey,
		protocol.SessionLocalDirectories{
			Directories: []string{directory, secondDirectory},
		},
	); err != nil {
		t.Fatalf("更新 Room-backed DM 本机目录失败: %v", err)
	}
	loaded, err = sessionService.GetLocalDirectories(desktopCtx, roomSessionKey)
	if err != nil || len(loaded.Directories) != 2 ||
		loaded.Directories[0] != directory ||
		loaded.Directories[1] != secondDirectory {
		t.Fatalf("读取 Room-backed DM 本机目录 = %+v, err=%v", loaded, err)
	}
}

func TestSessionServiceListsExternalIMSessions(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "个人微信助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	now := time.Now().UTC()
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWeixinPersonalSegment,
		"dm",
		"wx-user-1",
		"",
	)
	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err = store.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      agentValue.AgentID,
		ChannelType:  protocol.SessionChannelWeixinPersonal,
		ChatType:     protocol.RoomTypeDM,
		Status:       "closed",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "New Chat",
		MessageCount: 2,
		Options:      map[string]any{},
	}); err != nil {
		t.Fatalf("写入外部 IM session 失败: %v", err)
	}

	agentSessions, err := sessionService.ListAgentSessions(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取 agent sessions 失败: %v", err)
	}
	externalSession := findSessionByKey(agentSessions, sessionKey)
	if externalSession == nil {
		t.Fatalf("agent sessions 未包含外部 IM session: %+v", agentSessions)
	}
	if externalSession.RoomID != nil || externalSession.ConversationID != nil {
		t.Fatalf("外部 IM session 不应被伪装成普通 room conversation: %+v", externalSession)
	}
	if externalSession.ChannelType != protocol.SessionChannelWeixinPersonal {
		t.Fatalf("外部 IM channel_type 不正确: %+v", externalSession)
	}

	allSessions, err := sessionService.ListSessions(ctx)
	if err != nil {
		t.Fatalf("读取全部 sessions 失败: %v", err)
	}
	if findSessionByKey(allSessions, sessionKey) == nil {
		t.Fatalf("全部 sessions 未包含外部 IM session: %+v", allSessions)
	}
}

func TestTitleGenerationUpdatesExternalIMWorkspaceSession(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)

	agentValue, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{Name: "微信助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWeixinPersonalSegment,
		protocol.RoomTypeDM,
		"wx-user-1",
		"",
	)
	now := time.Now().UTC()
	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err = store.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      agentValue.AgentID,
		ChannelType:  protocol.SessionChannelWeixinPersonal,
		ChatType:     protocol.RoomTypeDM,
		Status:       "closed",
		CreatedAt:    now.Add(-time.Hour),
		LastActivity: now.Add(-time.Minute),
		Title:        "New Chat",
		MessageCount: 74,
		Options:      map[string]any{},
	}); err != nil {
		t.Fatalf("写入外部 IM session 失败: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "午餐建议"},
			},
		})
	}))
	defer server.Close()

	titleService := titlegensvc.NewService(
		staticTitleProvider{baseURL: server.URL},
		sessionService,
		nil,
		nil,
	)
	titleService.Schedule(context.Background(), titlegensvc.Request{
		OwnerUserID:              "__system__",
		SessionKey:               sessionKey,
		Content:                  "中午吃点啥好你觉得",
		SessionTitle:             "New Chat",
		SessionMessageCount:      74,
		ConversationMessageCount: -1,
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		updated, err := sessionService.GetSession(context.Background(), sessionKey)
		if err != nil {
			t.Fatalf("读取更新后的 IM session 失败: %v", err)
		}
		if updated.Title == "午餐建议" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("外部 IM session 标题未写回: %+v", updated)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
