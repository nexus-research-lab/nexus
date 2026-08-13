package server

import (
	"context"
	"os"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestAppServicesCloseReleasesOwnedDatabase(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	services, err := NewAppServices(cfg, nil)
	if err != nil {
		t.Fatalf("创建自有数据库 AppServices 失败: %v", err)
	}
	t.Cleanup(func() { _ = services.DB.Close() })
	if err = services.Close(context.Background()); err != nil {
		t.Fatalf("关闭自有数据库 AppServices 失败: %v", err)
	}
	if err = services.DB.Ping(); err == nil {
		t.Fatal("AppServices.Close 后数据库仍可用")
	}
	if err = os.Remove(cfg.DatabaseURL); err != nil {
		t.Fatalf("AppServices.Close 后数据库文件仍被占用: %v", err)
	}
}

func TestAppServicesClosePreservesBorrowedDatabase(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开外部数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	services := NewAppServicesWithDB(cfg, db, nil)
	if err = services.Close(context.Background()); err != nil {
		t.Fatalf("关闭借用数据库 AppServices 失败: %v", err)
	}
	if err = db.Ping(); err != nil {
		t.Fatalf("AppServices.Close 不应关闭外部数据库: %v", err)
	}
}

func TestAppServicesAutomationAcceptsAndProjectsDatabaseBackedAgentSessions(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	services := NewAppServicesWithDB(cfg, db, nil)
	defer func() { _ = services.Close(context.Background()) }()
	ctx := context.Background()
	agentValue, err := services.Core.Agent.CreateAgent(ctx, protocol.CreateRequest{Name: "Delivery Target"})
	if err != nil {
		t.Fatalf("创建接收 Agent 失败: %v", err)
	}
	dmContext, err := services.Core.Room.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建 Nexus DM 失败: %v", err)
	}
	roomContext, err := services.Core.Room.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "Delivery Room",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}

	intervalSeconds := 3600
	cases := []struct {
		name         string
		roomType     string
		conversation protocol.ConversationContextAggregate
	}{
		{name: "Nexus DM", roomType: protocol.RoomTypeDM, conversation: *dmContext},
		{name: "Room member", roomType: protocol.RoomTypeGroup, conversation: *roomContext},
	}
	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			sessionKey := protocol.BuildRoomAgentSessionKey(
				testCase.conversation.Conversation.ID,
				agentValue.AgentID,
				testCase.roomType,
			)
			if existing, _, findErr := files.FindSession([]string{agentValue.WorkspacePath}, sessionKey); findErr != nil {
				t.Fatalf("读取投影前 Session 失败: %v", findErr)
			} else if existing != nil {
				t.Fatalf("投递前不应已有 workspace Session: %#v", existing)
			}

			if _, err = services.Automation.CreateTask(ctx, automationdomain.CreateJobInput{
				Name:          "Deliver to " + testCase.name,
				AgentID:       agentValue.AgentID,
				Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &intervalSeconds},
				Instruction:   "Return a concise status.",
				ExecutionKind: automationdomain.ExecutionKindAgent,
				SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
				Delivery: automationdomain.DeliveryTarget{
					Mode:       automationdomain.DeliveryModeExplicit,
					Channel:    channels.ChannelTypeWebSocket,
					To:         sessionKey,
					SessionKey: sessionKey,
				},
				Source:  automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
				Enabled: false,
			}); err != nil {
				t.Fatalf("数据库 Session 应通过任务配置校验: %v", err)
			}

			resultText := "scheduled result for " + testCase.name
			if _, err = services.Channels.DeliverAutomationResult(
				ctx,
				agentValue.AgentID,
				resultText,
				channels.DeliveryTarget{
					Mode:       channels.DeliveryModeExplicit,
					Channel:    channels.ChannelTypeWebSocket,
					To:         sessionKey,
					SessionKey: sessionKey,
				},
				channels.AutomationDeliveryContext{
					JobID: "job-database-session",
					RunID: "run-database-session-" + testCase.roomType,
				},
			); err != nil {
				t.Fatalf("投递数据库 Session 失败: %v", err)
			}

			materialized, _, findErr := files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
			if findErr != nil {
				t.Fatalf("读取投影后 Session 失败: %v", findErr)
			}
			if materialized == nil || materialized.RoomID == nil || materialized.ConversationID == nil ||
				*materialized.RoomID != testCase.conversation.Room.ID ||
				*materialized.ConversationID != testCase.conversation.Conversation.ID {
				t.Fatalf("workspace Session 未保留数据库身份: %#v", materialized)
			}
			messages, readErr := services.Core.Session.GetSessionMessages(ctx, sessionKey)
			if readErr != nil {
				t.Fatalf("读取统一 Session 历史失败: %v", readErr)
			}
			if len(messages) != 1 {
				t.Fatalf("投递消息数 = %d，期望 1", len(messages))
			}
		})
	}
}
