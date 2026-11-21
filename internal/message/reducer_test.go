package message

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func reducerAssistantMessage(t *testing.T, messageID string, text string) sdkprotocol.ReceivedMessage {
	t.Helper()
	decoded, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "assistant", "uuid": "entry-" + messageID, "session_id": "sdk-session",
		"message": map[string]any{
			"id": messageID, "role": "assistant", "stop_reason": "end_turn",
			"content": []any{map[string]any{"type": "text", "text": text}},
		},
	})
	if err != nil {
		t.Fatalf("解码 assistant 消息失败: %v", err)
	}
	return decoded
}

// 轮次身份只来自边界事件：同一个归约器换轮后，新消息带新轮的全部身份，位置只写在回放的行上。
func TestReducerTakesRoundIdentityFromBoundaryEvents(t *testing.T) {
	reducer := NewReducer(MessageContext{SessionKey: "agent:a:ws:dm:c", AgentID: "a"})
	if reducer.Active() {
		t.Fatal("没有边界事件时不应有可用轮次")
	}

	reducer.BeginRound(RoundBoundary{
		Kind: RoundBoundaryUserInput, RoundID: "round-1", AgentRoundID: "agent-round-1", ParentID: "msg_user_round-1",
	})
	live := reducer.Reduce(reducerAssistantMessage(t, "msg-1", "第一轮")).DurableMessages
	if len(live) != 1 || live[0]["round_id"] != "round-1" || live[0]["agent_round_id"] != "agent-round-1" ||
		live[0]["parent_id"] != "msg_user_round-1" {
		t.Fatalf("第一轮身份 = %+v", live)
	}
	if _, positioned := live[0][TranscriptLineField]; positioned {
		t.Fatalf("实时行不应带 transcript 位置: %+v", live[0])
	}

	reducer.BeginRound(ImplicitRoundBoundary("round-2", "sdk-session"))
	if boundary := reducer.Boundary(); boundary.Kind != RoundBoundaryImplicit || boundary.ParentID != "msg_user_round-2" {
		t.Fatalf("隐式边界 = %+v", boundary)
	}
	replayed := reducer.ReduceAt(
		reducerAssistantMessage(t, "msg-2", "第二轮"),
		TranscriptPosition{Line: 7, Timestamp: 1795050000000},
	).DurableMessages
	if len(replayed) != 1 || replayed[0]["round_id"] != "round-2" || replayed[0]["parent_id"] != "msg_user_round-2" {
		t.Fatalf("第二轮身份 = %+v", replayed)
	}
	if _, inherited := replayed[0]["agent_round_id"]; inherited {
		t.Fatalf("新轮次继承了上一轮的 agent_round_id: %+v", replayed[0])
	}
	if replayed[0][TranscriptLineField] != 7 || replayed[0]["timestamp"] != int64(1795050000000) {
		t.Fatalf("回放行的位置 = %+v", replayed[0])
	}
}
