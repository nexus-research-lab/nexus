package launcher

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/sessionrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

type recordingDirectorySessionLister struct {
	items        []protocol.Session
	pages        map[string]*protocol.MessagePage
	calls        int
	pageKeys     []string
	pageRequests []sessionsvc.MessagePageRequest
}

func (lister *recordingDirectorySessionLister) ListDirectorySessions(
	context.Context,
) ([]protocol.Session, error) {
	lister.calls++
	return lister.items, nil
}

func (lister *recordingDirectorySessionLister) GetSessionMessagesPage(
	_ context.Context,
	sessionKey string,
	request sessionsvc.MessagePageRequest,
) (*protocol.MessagePage, error) {
	lister.pageKeys = append(lister.pageKeys, sessionKey)
	lister.pageRequests = append(lister.pageRequests, request)
	if page := lister.pages[sessionKey]; page != nil {
		return page, nil
	}
	return &protocol.MessagePage{Items: []protocol.Message{}}, nil
}

func TestLauncherQueryAndSuggestions(t *testing.T) {
	cfg := newLauncherTestConfig(t)
	migrateLauncherSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	roomService := roomsvc.NewService(cfg, agentService, roomrepo.NewSQLRepository("sqlite", db))
	sessionService := sessionsvc.NewService(cfg, agentService, sessionrepo.NewSQLRepository("sqlite", db))
	service := NewService(cfg, agentService, roomService, sessionService)

	ctx := context.Background()
	agentA := createLauncherAgent(t, agentService, ctx, "产品助手")
	agentB := createLauncherAgent(t, agentService, ctx, "设计助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "设计评审",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	queryResult, err := service.Query(ctx, "@产品助手 请梳理需求")
	if err != nil {
		t.Fatalf("解析 @agent 查询失败: %v", err)
	}
	if queryResult.ActionType != actionOpenAgentDM || queryResult.TargetID != agentA.AgentID {
		t.Fatalf("@agent 查询动作不正确: %+v", queryResult)
	}
	if queryResult.InitialMessage != "请梳理需求" {
		t.Fatalf("@agent 初始消息不正确: %s", queryResult.InitialMessage)
	}

	queryResult, err = service.Query(ctx, "#设计评审 进入房间")
	if err != nil {
		t.Fatalf("解析 #room 查询失败: %v", err)
	}
	if queryResult.ActionType != actionOpenRoom || queryResult.TargetID != roomContext.Room.ID {
		t.Fatalf("#room 查询动作不正确: %+v", queryResult)
	}

	queryResult, err = service.Query(ctx, "随便聊聊")
	if err != nil {
		t.Fatalf("解析普通查询失败: %v", err)
	}
	if queryResult.ActionType != actionOpenApp || queryResult.TargetID != "app" {
		t.Fatalf("open_app 动作不正确: %+v", queryResult)
	}

	suggestions, err := service.Suggestions(ctx)
	if err != nil {
		t.Fatalf("读取 Launcher 推荐失败: %v", err)
	}
	if len(suggestions.Agents) != 2 {
		t.Fatalf("推荐 agent 数量不正确: got=%d want=2", len(suggestions.Agents))
	}
	if len(suggestions.Rooms) != 1 {
		t.Fatalf("推荐 room 数量不正确: got=%d want=1", len(suggestions.Rooms))
	}
	if suggestions.Rooms[0].ID != roomContext.Room.ID {
		t.Fatalf("推荐 room 不正确: %+v", suggestions.Rooms[0])
	}
	if suggestions.Rooms[0].Type != "room" {
		t.Fatalf("推荐 room 类型不正确: %+v", suggestions.Rooms[0])
	}

	if _, err = roomService.UpdateConversation(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.UpdateConversationRequest{
		Title: "需求讨论",
	}); err != nil {
		t.Fatalf("更新 room 对话标题失败: %v", err)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊失败: %v", err)
	}
	if _, err = roomService.UpdateConversation(ctx, dmContext.Room.ID, dmContext.Conversation.ID, protocol.UpdateConversationRequest{
		Title: "产品私聊",
	}); err != nil {
		t.Fatalf("更新直聊标题失败: %v", err)
	}

	bootstrap, err := service.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("读取 launcher bootstrap 失败: %v", err)
	}
	if len(bootstrap.Conversations) == 0 {
		t.Fatalf("bootstrap conversations 不应为空")
	}
	assertContainsBootstrapRoomType(t, bootstrap.Rooms, roomContext.Room.ID, "room")
	assertContainsConversationTitle(t, bootstrap.Conversations, "需求讨论")
	assertContainsConversationTitle(t, bootstrap.Conversations, "产品私聊")
}

func TestLauncherBootstrapEnsuresMainAgentDefaultChat(t *testing.T) {
	cfg := newLauncherTestConfig(t)
	migrateLauncherSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	roomService := roomsvc.NewService(cfg, agentService, roomrepo.NewSQLRepository("sqlite", db))
	sessionService := sessionsvc.NewService(cfg, agentService, sessionrepo.NewSQLRepository("sqlite", db))
	service := NewService(cfg, agentService, roomService, sessionService)

	first, err := service.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("首次读取 launcher bootstrap 失败: %v", err)
	}
	if len(first.Agents) != 0 {
		t.Fatalf("主智能体不应进入普通 Agent 目录: %+v", first.Agents)
	}
	mainRoom := findBootstrapRoomByAgentID(first.Rooms, cfg.DefaultAgentID)
	if mainRoom == nil {
		t.Fatalf("bootstrap 应创建主智能体默认聊天: %+v", first.Rooms)
	}
	if mainRoom.RoomType != protocol.RoomTypeDM {
		t.Fatalf("主智能体默认聊天类型不正确: %+v", mainRoom)
	}
	if findBootstrapConversationByRoomID(first.Conversations, mainRoom.ID) == nil {
		t.Fatalf("主智能体默认聊天缺少可导航会话: %+v", first.Conversations)
	}

	second, err := service.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("再次读取 launcher bootstrap 失败: %v", err)
	}
	reusedRoom := findBootstrapRoomByAgentID(second.Rooms, cfg.DefaultAgentID)
	if reusedRoom == nil || reusedRoom.ID != mainRoom.ID {
		t.Fatalf("bootstrap 应幂等复用主智能体默认聊天: first=%+v second=%+v", mainRoom, reusedRoom)
	}
}

func TestLauncherBootstrapProjectsDirectorySessionMetadata(t *testing.T) {
	cfg := newLauncherTestConfig(t)
	migrateLauncherSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	roomService := roomsvc.NewService(cfg, agentService, roomrepo.NewSQLRepository("sqlite", db))
	sessionService := sessionsvc.NewService(cfg, agentService, sessionrepo.NewSQLRepository("sqlite", db))
	service := NewService(cfg, agentService, roomService, sessionService)

	now := time.Date(2026, 8, 17, 9, 30, 0, 0, time.UTC)
	lister := &recordingDirectorySessionLister{items: []protocol.Session{
		{
			SessionKey:   "agent:nexus:ws:dm:catalog-only",
			AgentID:      cfg.DefaultAgentID,
			ChannelType:  "ws",
			ChatType:     protocol.RoomTypeDM,
			CreatedAt:    now,
			LastActivity: now,
			Title:        "metadata only",
			MessageCount: 9,
		},
	}}
	service.session = lister

	bootstrap, err := service.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("读取 launcher bootstrap 失败: %v", err)
	}
	if lister.calls != 1 {
		t.Fatalf("bootstrap 应仅批量读取一次目录 metadata: got=%d want=1", lister.calls)
	}
	item := findBootstrapConversationBySessionKey(
		bootstrap.Conversations,
		"agent:nexus:ws:dm:catalog-only",
	)
	if item == nil || item.Title != "metadata only" || item.MessageCount != 9 {
		t.Fatalf("bootstrap 未直接投影目录 metadata: %+v", bootstrap.Conversations)
	}
}

func TestLauncherBootstrapUsesBoundedLatestReplyPreview(t *testing.T) {
	cfg := newLauncherTestConfig(t)
	migrateLauncherSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	roomService := roomsvc.NewService(cfg, agentService, roomrepo.NewSQLRepository("sqlite", db))
	service := NewService(
		cfg,
		agentService,
		roomService,
		sessionsvc.NewService(cfg, agentService, sessionrepo.NewSQLRepository("sqlite", db)),
	)
	ctx := context.Background()
	dmContext, err := roomService.EnsureDirectRoom(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("创建主智能体直聊失败: %v", err)
	}
	sessionKey := protocol.BuildRoomAgentSessionKey(
		dmContext.Conversation.ID,
		cfg.DefaultAgentID,
		protocol.RoomTypeDM,
	)
	now := time.Date(2026, 8, 27, 9, 30, 0, 0, time.UTC)
	lister := &recordingDirectorySessionLister{
		items: []protocol.Session{{
			SessionKey:     sessionKey,
			AgentID:        cfg.DefaultAgentID,
			RoomID:         &dmContext.Room.ID,
			ConversationID: &dmContext.Conversation.ID,
			ChannelType:    "ws",
			ChatType:       protocol.RoomTypeDM,
			CreatedAt:      now,
			LastActivity:   now,
			Title:          "会话标题不能冒充消息",
			MessageCount:   2,
		}},
		pages: map[string]*protocol.MessagePage{
			sessionKey: {
				Items: []protocol.Message{{
					"role": "assistant",
					"content": []map[string]any{{
						"type": "text",
						"text": "  最新回复\n包含多行内容  ",
					}},
				}},
			},
		},
	}
	service.session = lister

	bootstrap, err := service.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("读取 launcher bootstrap 失败: %v", err)
	}
	item := findBootstrapConversationBySessionKey(bootstrap.Conversations, sessionKey)
	if item == nil || item.LastReplyPreview != "最新回复 包含多行内容" {
		t.Fatalf("bootstrap 最新回复预览不正确: %+v", item)
	}
	if len(lister.pageKeys) != 1 || lister.pageKeys[0] != sessionKey {
		t.Fatalf("bootstrap 最新回复读取目标不正确: %+v", lister.pageKeys)
	}
	if len(lister.pageRequests) != 1 || lister.pageRequests[0].Limit != 2 {
		t.Fatalf("bootstrap 必须只读取最近两个 round: %+v", lister.pageRequests)
	}
}

func TestPreviewSessionKeyRoutesGroupToSharedHistory(t *testing.T) {
	group := BootstrapConversation{
		SessionKey:     "member-session",
		RoomType:       protocol.RoomTypeGroup,
		ConversationID: "conversation-1",
	}
	if got := previewSessionKey(group); got != protocol.BuildRoomSharedSessionKey("conversation-1") {
		t.Fatalf("previewSessionKey(group) = %q, want shared history", got)
	}

	dm := BootstrapConversation{SessionKey: "dm-session", RoomType: protocol.RoomTypeDM}
	if got := previewSessionKey(dm); got != dm.SessionKey {
		t.Fatalf("previewSessionKey(dm) = %q, want member history", got)
	}
}

func TestLauncherBootstrapIgnoresCorruptConversationHistory(t *testing.T) {
	cfg := newLauncherTestConfig(t)
	migrateLauncherSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	roomService := roomsvc.NewService(cfg, agentService, roomrepo.NewSQLRepository("sqlite", db))
	sessionService := sessionsvc.NewService(cfg, agentService, sessionrepo.NewSQLRepository("sqlite", db))
	service := NewService(cfg, agentService, roomService, sessionService)
	ctx := context.Background()

	agentValue := createLauncherAgent(t, agentService, ctx, "历史损坏隔离助手")
	dmContext, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建测试直聊失败: %v", err)
	}
	sessionKey := protocol.BuildRoomAgentSessionKey(
		dmContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeDM,
	)
	overlayPath := workspacestore.New(cfg.WorkspacePath).SessionOverlayPath(
		agentValue.WorkspacePath,
		sessionKey,
	)
	if err = os.MkdirAll(filepath.Dir(overlayPath), 0o700); err != nil {
		t.Fatalf("创建测试历史目录失败: %v", err)
	}
	corruptHistory := strings.Repeat("{not-valid-json}\n", 4096)
	if err = os.WriteFile(overlayPath, []byte(corruptHistory), 0o600); err != nil {
		t.Fatalf("写入损坏历史夹具失败: %v", err)
	}

	bootstrap, err := service.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("单个会话历史损坏不应拖垮 launcher bootstrap: %v", err)
	}
	if findBootstrapConversationBySessionKey(bootstrap.Conversations, sessionKey) == nil {
		t.Fatalf("损坏历史不应让对应 metadata 会话消失: %+v", bootstrap.Conversations)
	}
	if len(bootstrap.Rooms) < 2 {
		t.Fatalf("损坏历史不应影响其他 Room 目录: %+v", bootstrap.Rooms)
	}
}

func assertContainsBootstrapRoomType(
	t *testing.T,
	items []BootstrapRoom,
	roomID string,
	roomType string,
) {
	t.Helper()

	for _, item := range items {
		if item.ID == roomID && item.RoomType == roomType {
			return
		}
	}
	t.Fatalf("bootstrap room 类型缺失: room_id=%s room_type=%s items=%+v", roomID, roomType, items)
}

func assertContainsConversationTitle(
	t *testing.T,
	items []BootstrapConversation,
	title string,
) {
	t.Helper()

	for _, item := range items {
		if item.Title == title {
			return
		}
	}
	t.Fatalf("bootstrap conversations 缺少标题 %q: %+v", title, items)
}

func findBootstrapRoomByAgentID(
	items []BootstrapRoom,
	agentID string,
) *BootstrapRoom {
	for index := range items {
		if items[index].DMTargetAgentID == agentID {
			return &items[index]
		}
	}
	return nil
}

func findBootstrapConversationByRoomID(
	items []BootstrapConversation,
	roomID string,
) *BootstrapConversation {
	for index := range items {
		if items[index].RoomID == roomID {
			return &items[index]
		}
	}
	return nil
}

func TestBuildBootstrapConversationsIncludesNavigationFields(t *testing.T) {
	roomID := "room-1"
	conversationID := "conversation-1"
	now := time.Date(2026, 5, 20, 9, 30, 0, 0, time.UTC)
	externalSessionKey := protocol.BuildAgentSessionKey(
		"amy",
		protocol.SessionChannelWeixinPersonalSegment,
		"dm",
		"wx-user-1",
		"",
	)

	items := buildBootstrapConversations([]protocol.Session{
		{
			SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
			AgentID:        "amy",
			RoomID:         &roomID,
			ConversationID: &conversationID,
			ChannelType:    "ws",
			ChatType:       protocol.RoomTypeGroup,
			CreatedAt:      now,
			LastActivity:   now,
			Title:          "room",
			MessageCount:   2,
		},
		{
			SessionKey:   externalSessionKey,
			AgentID:      "amy",
			ChannelType:  protocol.SessionChannelWeixinPersonal,
			ChatType:     protocol.RoomTypeDM,
			CreatedAt:    now.Add(time.Minute),
			LastActivity: now.Add(time.Minute),
			Title:        "New Chat",
			MessageCount: 4,
		},
	}, map[string]string{roomID: protocol.RoomTypeGroup})

	if len(items) != 2 {
		t.Fatalf("bootstrap conversations 数量不正确: %+v", items)
	}
	if items[0].ChannelType != "ws" || items[0].RoomType != "room" {
		t.Fatalf("bootstrap conversation 应携带导航语义: %+v", items[0])
	}
	externalItem := findBootstrapConversationBySessionKey(items, externalSessionKey)
	if externalItem == nil {
		t.Fatalf("bootstrap conversations 缺少外部 IM session: %+v", items)
	}
	if externalItem.RoomID != "" || externalItem.ConversationID != "" {
		t.Fatalf("外部 IM session 不应伪装为普通 room conversation: %+v", externalItem)
	}
	if externalItem.AgentID != "amy" || externalItem.ChannelType != protocol.SessionChannelWeixinPersonal {
		t.Fatalf("外部 IM session 投影字段不正确: %+v", externalItem)
	}
}

func findBootstrapConversationBySessionKey(
	items []BootstrapConversation,
	sessionKey string,
) *BootstrapConversation {
	for index := range items {
		if items[index].SessionKey == sessionKey {
			return &items[index]
		}
	}
	return nil
}

func createLauncherAgent(
	t *testing.T,
	service *agentsvc.Service,
	ctx context.Context,
	name string,
) *protocol.Agent {
	t.Helper()

	item, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: name})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	return item
}

func newLauncherTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, ".nexus"))
	t.Setenv("NEXUS_CONFIG_DIR", "")
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18012,
		ProjectName:    "nexus-launcher-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateLauncherSQLite(t *testing.T, databaseURL string) {
	t.Helper()
	handlertest.MigrateSQLiteFromDir(t, databaseURL, launcherMigrationDir(t))
}

func launcherMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
