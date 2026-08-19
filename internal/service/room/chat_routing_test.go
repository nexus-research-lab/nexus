package room

import (
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestLatestActiveRootRoundAgentIDsPrefersRegistrationSequence(t *testing.T) {
	service := &RealtimeService{activeRounds: make(map[string]*activeRoomRound)}
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-latest-root")
	register := func(roundID string, agentID string) {
		service.registerRound(&activeRoomRound{
			SessionKey:     sessionKey,
			ConversationID: "conversation-latest-root",
			RoundID:        roundID,
			RootRoundID:    roundID,
			Slots: map[string]*activeRoomSlot{
				agentID: {
					AgentID:      agentID,
					AgentRoundID: roundID + "-agent",
					Status:       "running",
					TimestampMS:  100,
				},
			},
		})
	}

	// root id 的字典序故意与注册顺序相反，确保并列时间戳不参与主判定。
	register("a-earlier-root", "agent-earlier")
	register("z-later-root", "agent-later")

	got := service.latestActiveRootRoundAgentIDs(sessionKey, "conversation-latest-root")
	if want := []string{"agent-later"}; !slices.Equal(got, want) {
		t.Fatalf("最近活跃 root 目标 = %+v, want %+v", got, want)
	}
}

func TestResolveChatTargetAgentIDsUsesExplicitTargets(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-tom"},
		},
	}
	targets, resolution, err := resolveChatTargetAgentIDs(
		ChatRequest{Content: "没有 mention 也要给 Amy", TargetAgentIDs: []string{"agent-amy", "agent-amy", " "}},
		contextValue,
		map[string]string{"agent-amy": "Amy", "agent-tom": "Tom"},
	)
	if err != nil {
		t.Fatalf("显式 Room 目标解析失败: %v", err)
	}
	if resolution != "explicit_target" || len(targets) != 1 || targets[0] != "agent-amy" {
		t.Fatalf("显式 Room 目标解析不正确: targets=%+v resolution=%s", targets, resolution)
	}
}

func TestNewRoomUserMessagePersistsResolvedTargets(t *testing.T) {
	message := newRoomUserMessage(
		ChatRequest{RoundID: "round-targets", UserMessageID: "message-targets", Content: "只调整 Agent1 的回复"},
		"room:group:conversation-targets",
		"room-targets",
		"conversation-targets",
		nil,
		[]string{"agent-1"},
		protocol.ChatDeliveryPolicyGuide,
	)
	targets, ok := message["target_agent_ids"].([]string)
	if !ok || len(targets) != 1 || targets[0] != "agent-1" {
		t.Fatalf("target_agent_ids = %#v, want resolved target", message["target_agent_ids"])
	}
}

func TestPromoteScriptedRoomHostMessageUsesHostIdentity(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{HostAgentID: "agent-nexus"},
	}
	messageValue := newRoomUserMessage(
		ChatRequest{
			RoundID:             "round-onboarding",
			UserMessageID:       "message-onboarding",
			Content:             "@研究顾问 开始评审",
			ScriptedHostMessage: true,
		},
		"room:group:conversation-onboarding",
		"room-onboarding",
		"conversation-onboarding",
		nil,
		[]string{"agent-researcher"},
		protocol.ChatDeliveryPolicyQueue,
	)
	err := promoteScriptedRoomHostMessage(
		ChatRequest{
			Content:             "@研究顾问 开始评审",
			ScriptedHostMessage: true,
		},
		contextValue,
		map[string]*protocol.Agent{
			"agent-nexus": {AgentID: "agent-nexus"},
		},
		nil,
		nil,
		[]string{"agent-researcher"},
		messageValue,
	)
	if err != nil {
		t.Fatalf("主持人开场消息转换失败: %v", err)
	}
	if messageValue["role"] != "assistant" || messageValue["agent_id"] != "agent-nexus" {
		t.Fatalf("主持人开场消息身份不正确: %+v", messageValue)
	}
	if messageValue["is_complete"] != true || messageValue["stop_reason"] != "scripted" {
		t.Fatalf("主持人开场消息终态不正确: %+v", messageValue)
	}
	blocks, ok := messageValue["content"].([]map[string]any)
	if !ok || len(blocks) != 1 || blocks[0]["text"] != "@研究顾问 开始评审" {
		t.Fatalf("主持人开场消息内容块不正确: %+v", messageValue["content"])
	}
}

func TestPromoteScriptedRoomHostMessageRejectsNonEmptyConversation(t *testing.T) {
	messageValue := protocol.Message{}
	err := promoteScriptedRoomHostMessage(
		ChatRequest{ScriptedHostMessage: true},
		&protocol.ConversationContextAggregate{
			Room: protocol.RoomRecord{HostAgentID: "agent-nexus"},
		},
		map[string]*protocol.Agent{
			"agent-nexus": {AgentID: "agent-nexus"},
		},
		[]protocol.Message{{"role": "user", "content": "existing"}},
		nil,
		[]string{"agent-researcher"},
		messageValue,
	)
	if err == nil || !strings.Contains(err.Error(), "first room message") {
		t.Fatalf("非空会话应拒绝主持人开场消息: %v", err)
	}
}

func TestResolveChatTargetAgentIDsRejectsNonMemberTarget(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
		},
	}
	_, _, err := resolveChatTargetAgentIDs(
		ChatRequest{Content: "整理一下", TargetAgentIDs: []string{"agent-outsider"}},
		contextValue,
		map[string]string{"agent-amy": "Amy"},
	)
	if err == nil || !strings.Contains(err.Error(), "not a room member") {
		t.Fatalf("非成员目标应被拒绝: %v", err)
	}
}

func TestBuildPublicMentionSlotKeepsPublicTriggerMessage(t *testing.T) {
	slot := buildPublicMentionSlot(
		&protocol.ConversationContextAggregate{
			Room:         protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
			Conversation: protocol.ConversationRecord{ID: "conversation-1"},
		},
		protocol.SessionRecord{ID: "session-devin"},
		&protocol.Agent{AgentID: "agent-devin", WorkspacePath: t.TempDir()},
		publicMentionWake{
			SourceAgentID: "agent-amy",
			TargetAgentID: "agent-devin",
			Content:       "@Devin @sam 谁先来？",
			MessageID:     "message-1",
		},
		"round-1",
		"message-slot-1",
		0,
	)

	if slot.Trigger.TriggerType != "public_mention" ||
		slot.Trigger.SourceAgentID != "agent-amy" ||
		slot.Trigger.TargetAgentID != "agent-devin" ||
		slot.Trigger.MessageID != "message-1" ||
		slot.Trigger.Content != "@Devin @sam 谁先来？" {
		t.Fatalf("公区 @ slot 应只保留可直接渲染成消息行的触发信息: %+v", slot.Trigger)
	}
}
