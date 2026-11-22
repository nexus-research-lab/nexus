// INPUT: 经 Room 实时服务受理的公区用户输入、chat_ack、durable 用户行广播与落盘公区历史。
// OUTPUT: 同一输入在 ACK、实时行与历史行上带同一个 input_seq，刷新与换新服务实例后不变。
// POS: Room 用户输入受理游标的端到端合同测试。
package realtime_test

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestRoomUserRowSharesInputSeqAcrossAckLiveRowAndHistory(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "受理游标测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)

	// 每次调用都装配全新的实时服务与连接，只共用落盘状态，等价于进程重启后的宿主。
	newHost := func(senderKey string) (*realtimesvc.Service, *realtimeTestSender) {
		permission := permissionctx.NewContext()
		service := realtimesvc.NewServiceWithFactory(cfg, roomService, agentService, runtimectx.NewManager(), permission, &fakeRoomFactory{})
		sender := newRealtimeTestSender(senderKey)
		permission.BindSession(sharedSessionKey, sender)
		return service, sender
	}
	// accept 受理一条公区输入，要求 ACK 与实时用户行给出同一身份与游标。
	accept := func(service *realtimesvc.Service, sender *realtimeTestSender, content string) (string, int64) {
		t.Helper()
		if err := service.HandleChat(ctx, realtimesvc.ChatRequest{
			SessionKey:                  sharedSessionKey,
			RoomID:                      roomContext.Room.ID,
			ConversationID:              roomContext.Conversation.ID,
			ClientMessageID:             "local-" + content,
			Content:                     content,
			TrustedConfigurationContext: true,
		}); err != nil {
			t.Fatalf("HandleChat 失败: %v", err)
		}
		events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
			return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
		})
		var userMessageID, liveMessageID string
		var ackSeq, liveSeq int64
		for _, event := range events {
			switch {
			case event.EventType == protocol.EventTypeChatAck:
				userMessageID, _ = event.Data["user_message_id"].(string)
				ackSeq = protocol.Int64FromAny(event.Data[protocol.InputSeqField])
			case event.EventType == protocol.EventTypeMessage && event.Data["role"] == "user":
				liveMessageID, _ = event.Data["message_id"].(string)
				liveSeq = protocol.Int64FromAny(event.Data[protocol.InputSeqField])
			}
		}
		if ackSeq <= 0 || liveSeq != ackSeq || liveMessageID != userMessageID {
			t.Fatalf("ACK 与实时用户行必须给出同一身份与游标: ack=(%s,%d) live=(%s,%d)", userMessageID, ackSeq, liveMessageID, liveSeq)
		}
		return userMessageID, ackSeq
	}
	live := map[string]int64{}
	// assertHistory 每次都用全新的历史 store 读取落盘公区历史。
	assertHistory := func(stage string) {
		t.Helper()
		rows, err := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath).ReadMessages(
			roomContext.Room.OwnerUserID,
			roomContext.Conversation.ID,
			nil,
		)
		if err != nil {
			t.Fatalf("%s: 读取 Room 公区历史失败: %v", stage, err)
		}
		got := map[string]int64{}
		for _, row := range rows {
			if row["role"] == "user" {
				messageID, _ := row["message_id"].(string)
				got[messageID] = protocol.Int64FromAny(row[protocol.InputSeqField])
			}
		}
		if len(got) != len(live) {
			t.Fatalf("%s: 历史用户行 = %v，实时受理 = %v", stage, got, live)
		}
		for messageID, inputSeq := range live {
			if got[messageID] != inputSeq {
				t.Fatalf("%s: %s 的历史游标 = %d，受理时是 %d", stage, messageID, got[messageID], inputSeq)
			}
		}
	}

	service, sender := newHost("room-input-cursor-1")
	firstID, firstSeq := accept(service, sender, "先记一下这个背景")
	live[firstID] = firstSeq
	assertHistory("受理后读取")

	secondID, secondSeq := accept(service, sender, "再补充一条")
	if secondSeq <= firstSeq {
		t.Fatalf("后受理的输入游标必须更大: first=%d second=%d", firstSeq, secondSeq)
	}
	live[secondID] = secondSeq
	assertHistory("追加一条后刷新")

	restarted, restartedSender := newHost("room-input-cursor-2")
	assertHistory("换新服务实例后读取")
	thirdID, thirdSeq := accept(restarted, restartedSender, "重启后的第三条")
	// 这里只换服务实例，分配器仍是同一进程的；真正跨进程的先后取决于宿主时钟（见 protocol.NewInputSeq）。
	if thirdSeq <= secondSeq {
		t.Fatalf("换新服务实例后受理的输入游标必须大于已落盘的游标: second=%d third=%d", secondSeq, thirdSeq)
	}
	live[thirdID] = thirdSeq
	assertHistory("重启后再受理一条")
}
