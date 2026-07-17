package room

import (
	"context"
	"strings"
	"testing"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestAnnotatePublicAssistantMessageSeparatesDisplayMentionFromDefaultHandoff(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: "conversation-intent"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-source"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-devin"},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-source", Name: "Source"},
			{AgentID: "agent-amy", Name: "Amy"},
			{AgentID: "agent-devin", Name: "Devin"},
		},
	}
	roundValue := &activeRoomRound{
		Context: contextValue, ConversationID: contextValue.Conversation.ID,
		RoomID: "room-intent", RootRoundID: "root-intent",
	}
	slot := &activeRoomSlot{AgentID: "agent-source", AgentRoundID: "source-round"}
	message := protocol.Message{
		"message_id":  "message-intent",
		"role":        "assistant",
		"is_complete": true,
		// runtime 传入的旧 annotation 不能绕过服务端的单目标选择。
		"agent_mentions": []protocol.AgentMention{{
			AgentID: "agent-devin", HandoffID: "runtime-forged-handoff",
		}},
		"content": []map[string]any{{
			"type": "text", "text": "先请 @Amy 处理，@Devin 作为展示候选。",
		}},
	}
	service := &RealtimeService{}
	if err := service.annotatePublicAssistantMessage(roundValue, slot, message); err != nil {
		t.Fatal(err)
	}
	mentions := protocolAgentMentions(message["agent_mentions"])
	if len(mentions) != 2 || mentions[0].HandoffID == "" || mentions[1].HandoffID != "" {
		t.Fatalf("默认单目标 handoff 标注不正确: %+v", mentions)
	}
	if wakes := publicMentionWakesFromMessage(roundValue, slot, message, roomdomain.ExtractAssistantResultText(message)); len(wakes) != 1 || wakes[0].TargetAgentID != "agent-amy" {
		t.Fatalf("展示 mention 不应唤醒目标: %+v", wakes)
	}
}

func TestAnnotatePublicAssistantMessageRequiresExplicitFanoutMarker(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: "conversation-fanout"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-source"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-devin"},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-source", Name: "Source"},
			{AgentID: "agent-amy", Name: "Amy"},
			{AgentID: "agent-devin", Name: "Devin"},
		},
	}
	roundValue := &activeRoomRound{
		Context: contextValue, ConversationID: contextValue.Conversation.ID,
		RoomID: "room-fanout", RootRoundID: "root-fanout",
	}
	slot := &activeRoomSlot{AgentID: "agent-source", AgentRoundID: "source-round"}
	message := protocol.Message{
		"message_id":  "message-fanout",
		"role":        "assistant",
		"is_complete": true,
		"content": []map[string]any{{
			"type": "text", "text": "请 @Amy 和 @Devin 并行处理。<nexus_room_fanout/>",
		}},
	}
	service := &RealtimeService{}
	if err := service.annotatePublicAssistantMessage(roundValue, slot, message); err != nil {
		t.Fatal(err)
	}
	content := roomdomain.ExtractAssistantResultText(message)
	if strings.Contains(content, roomdomain.FanoutMarker) {
		t.Fatalf("fanout 控制标记不应进入正文: %q", content)
	}
	mentions := protocolAgentMentions(message["agent_mentions"])
	if len(mentions) != 2 || mentions[0].HandoffID == "" || mentions[1].HandoffID == "" {
		t.Fatalf("显式 fanout 应为全部目标写入 handoff: %+v", mentions)
	}
	if wakes := publicMentionWakesFromMessage(roundValue, slot, message, content); len(wakes) != 2 {
		t.Fatalf("显式 fanout 应唤醒两个目标: %+v", wakes)
	}
}

func TestPublicHandoffAdmissionDetectsCycle(t *testing.T) {
	edges := []workspacestore.RoomPublicHandoff{
		{SourceAgentID: "agent-a", TargetAgentID: "agent-b"},
		{SourceAgentID: "agent-b", TargetAgentID: "agent-c"},
	}
	if !roomPublicHandoffCreatesCycle(edges, "agent-c", "agent-a") {
		t.Fatal("应拒绝会回到 source 的 root handoff 环")
	}
	if roomPublicHandoffCreatesCycle(edges, "agent-c", "agent-d") {
		t.Fatal("无环的新目标不应被拒绝")
	}
}

func TestPublicHandoffAdmissionRejectsRecordedCycleAndRootOverflow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", root)
	conversationID := "conversation-admission-guard"
	store := workspacestore.NewRoomPublicHandoffStore(root)
	detect := func(handoff workspacestore.RoomPublicHandoff) {
		t.Helper()
		if _, _, err := store.Detect(handoff); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
			t.Fatal(err)
		}
	}
	detect(workspacestore.RoomPublicHandoff{
		HandoffID: "rh-cycle-forward", ConversationID: conversationID, RootRoundID: "root-guard",
		SourceMessageID: "message-forward", SourceAgentID: "agent-a", TargetAgentID: "agent-b",
	})
	detect(workspacestore.RoomPublicHandoff{
		HandoffID: "rh-cycle-back", ConversationID: conversationID, RootRoundID: "root-guard",
		SourceMessageID: "message-back", SourceAgentID: "agent-b", TargetAgentID: "agent-a",
	})
	service := &RealtimeService{publicHandoffs: store}
	parent := &activeRoomRound{ConversationID: conversationID, RootRoundID: "root-guard"}
	accepted, err := service.admitPublicMentionWakes(context.Background(), parent, []publicMentionWake{{
		HandoffID: "rh-cycle-back", QueueSource: protocol.InputQueueSourceAgentPublicMention,
		SourceAgentID: "agent-b", TargetAgentID: "agent-a",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 0 {
		t.Fatalf("已记录的 reciprocal edge 仍必须经过 cycle guard: %+v", accepted)
	}
	cycle, ok, err := store.Get(conversationID, "rh-cycle-back")
	if err != nil || !ok || cycle.Status != "error" {
		t.Fatalf("cycle handoff 应收口为 error: handoff=%+v ok=%v err=%v", cycle, ok, err)
	}

	for index := 0; index < roomMaxRootHandoffs; index++ {
		detect(workspacestore.RoomPublicHandoff{
			HandoffID:       "rh-overflow-" + string(rune('a'+index)),
			ConversationID:  conversationID,
			RootRoundID:     "root-overflow",
			SourceMessageID: "message-overflow-" + string(rune('a'+index)),
			SourceAgentID:   "agent-source-" + string(rune('a'+index)),
			TargetAgentID:   "agent-target-" + string(rune('a'+index)),
		})
	}
	overflowID := "rh-overflow-new"
	detect(workspacestore.RoomPublicHandoff{
		HandoffID: overflowID, ConversationID: conversationID, RootRoundID: "root-overflow",
		SourceMessageID: "message-overflow-new", SourceAgentID: "agent-source-new", TargetAgentID: "agent-target-new",
	})
	accepted, err = service.admitPublicMentionWakes(context.Background(), &activeRoomRound{
		ConversationID: conversationID, RootRoundID: "root-overflow",
	}, []publicMentionWake{{
		HandoffID: overflowID, QueueSource: protocol.InputQueueSourceAgentPublicMention,
		SourceAgentID: "agent-source-new", TargetAgentID: "agent-target-new",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 0 {
		t.Fatalf("root handoff 超限后不应继续接受新边: %+v", accepted)
	}
}
