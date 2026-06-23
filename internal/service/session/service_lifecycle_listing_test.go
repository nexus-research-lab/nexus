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

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	sessionService.SetRuntimeManager(runtimectx.NewManager())

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "测试会话助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "launcher-app-"+agentA.AgentID, "")
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

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}
	seedRoomConversationMessages(t, cfg, dmContext.Conversation.ID)

	sessions, err := sessionService.ListSessions(ctx)
	if err != nil {
		t.Fatalf("列出 sessions 失败: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatalf("session 列表未合并 room 视图: got=%d", len(sessions))
	}

	agentSessions, err := sessionService.ListAgentSessions(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取 agent sessions 失败: %v", err)
	}
	if len(agentSessions) < 2 {
		t.Fatalf("agent sessions 数量不正确: got=%d", len(agentSessions))
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
	if messagePage.Items[0]["message_id"] != "round_1" {
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

	if err = sessionService.DeleteSession(ctx, dmKey); err != nil {
		t.Fatalf("删除 session 失败: %v", err)
	}
	if _, err = sessionService.GetSession(ctx, dmKey); err == nil {
		t.Fatal("删除后不应还能读取到 session")
	}
	if _, err = os.Stat(sessionTranscriptFilePath(agentA.WorkspacePath, dmSessionID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除 session 后 transcript 仍残留: %v", err)
	}
}

func TestSessionServiceListsExternalIMSessions(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
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

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
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
