package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestProjectConversationTurnsDMSingleAgent(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "msg_user_1", "round_id": "round_1", "agent_round_id": "agent_round_1", "role": "user", "content": "hello", "timestamp": int64(100)},
		{"message_id": "msg_a_1", "round_id": "round_1", "agent_round_id": "agent_round_1", "agent_id": "agent_a", "role": "assistant", "content": "hi", "timestamp": int64(200)},
		{"message_id": "result_1", "round_id": "round_1", "agent_round_id": "agent_round_1", "agent_id": "agent_a", "role": "result", "subtype": "success", "duration_ms": int64(5), "timestamp": int64(300)},
	}
	turns := ProjectConversationTurns(rows, false, nil)
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	turn := turns[0]
	if turn.RoundID != "round_1" || turn.Status != "finished" {
		t.Fatalf("unexpected turn: %+v", turn)
	}
	if turn.UserMessage == nil || turn.UserMessage.MessageID != "msg_user_1" {
		t.Fatalf("unexpected user message: %+v", turn.UserMessage)
	}
	if len(turn.AgentSlots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(turn.AgentSlots))
	}
	slot := turn.AgentSlots[0]
	if slot.AgentID != "agent_a" || slot.AgentRoundID != "agent_round_1" || slot.Status != "finished" {
		t.Fatalf("unexpected slot: %+v", slot)
	}
	if len(slot.AssistantMessages) != 1 || slot.AssistantMessages[0].RoundID != "round_1" {
		t.Fatalf("unexpected assistant messages: %+v", slot.AssistantMessages)
	}
	if slot.ResultSummary == nil || slot.ResultSummary["subtype"] != "success" {
		t.Fatalf("unexpected result summary: %+v", slot.ResultSummary)
	}
}

func TestProjectConversationTurnsRoomMultiAgent(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "msg_user_1", "round_id": "round_1", "role": "user", "content": "hello all", "timestamp": int64(100)},
		{"message_id": "msg_a", "round_id": "round_1", "agent_round_id": "ar_a", "agent_id": "agent_a", "role": "assistant", "content": "a", "timestamp": int64(200)},
		{"message_id": "msg_b", "round_id": "round_1", "agent_round_id": "ar_b", "agent_id": "agent_b", "role": "assistant", "content": "b", "timestamp": int64(210)},
		{"message_id": "result_a", "round_id": "round_1", "agent_round_id": "ar_a", "agent_id": "agent_a", "role": "result", "subtype": "success", "timestamp": int64(300)},
		{"message_id": "result_b", "round_id": "round_1", "agent_round_id": "ar_b", "agent_id": "agent_b", "role": "result", "subtype": "interrupted", "timestamp": int64(310)},
	}
	turns := ProjectConversationTurns(rows, true, nil)
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	turn := turns[0]
	if len(turn.AgentSlots) != 2 {
		t.Fatalf("expected 2 slots, got %+v", turn.AgentSlots)
	}
	if turn.AgentSlots[0].Status != "finished" || turn.AgentSlots[1].Status != "interrupted" {
		t.Fatalf("unexpected slot statuses: %+v", turn.AgentSlots)
	}
	if turn.Status != "interrupted" {
		t.Fatalf("expected turn interrupted, got %s", turn.Status)
	}
}

func TestProjectConversationTurnsLegacySuffixNormalization(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "round_1", "round_id": "round_1", "role": "user", "content": "legacy", "timestamp": int64(100)},
		{"message_id": "msg_a", "round_id": "round_1:agent_a", "agent_id": "agent_a", "role": "assistant", "content": "a", "timestamp": int64(200)},
		{"message_id": "msg_b", "round_id": "round_1:agent_b", "agent_id": "agent_b", "role": "assistant", "content": "b", "timestamp": int64(210)},
	}
	turns := ProjectConversationTurns(rows, true, nil)
	if len(turns) != 1 {
		t.Fatalf("expected legacy rows to collapse into 1 turn, got %d: %+v", len(turns), turns)
	}
	turn := turns[0]
	if turn.RoundID != "round_1" {
		t.Fatalf("unexpected round id: %s", turn.RoundID)
	}
	if len(turn.AgentSlots) != 2 {
		t.Fatalf("expected 2 slots, got %+v", turn.AgentSlots)
	}
	// 旧后缀值直接当稳定 agent_round_id。
	if turn.AgentSlots[0].AgentRoundID != "round_1:agent_a" {
		t.Fatalf("unexpected agent_round_id: %s", turn.AgentSlots[0].AgentRoundID)
	}
	for _, slot := range turn.AgentSlots {
		for _, message := range slot.AssistantMessages {
			if message.RoundID != "round_1" {
				t.Fatalf("message round_id must be root round, got %s", message.RoundID)
			}
		}
	}
}

func TestProjectConversationTurnsActiveRound(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "msg_user_1", "round_id": "round_1", "role": "user", "content": "hi", "timestamp": int64(100)},
	}
	turns := ProjectConversationTurns(rows, false, []string{"round_1"})
	if turns[0].Status != "running" {
		t.Fatalf("expected running, got %s", turns[0].Status)
	}
}

func TestPaginateConversationTurns(t *testing.T) {
	turns := []protocol.ConversationTurn{
		{RoundID: "r1", CreatedAt: 1},
		{RoundID: "r2", CreatedAt: 2},
		{RoundID: "r3", CreatedAt: 3},
		{RoundID: "r4", CreatedAt: 4},
	}
	page := PaginateConversationTurns(turns, 2, "", "", false)
	if len(page.Turns) != 2 || page.Turns[0].RoundID != "r3" {
		t.Fatalf("unexpected latest page: %+v", page)
	}
	if page.NextBeforeRoundID != "r3" {
		t.Fatalf("unexpected cursor: %q", page.NextBeforeRoundID)
	}

	older := PaginateConversationTurns(turns, 2, "r3", "", false)
	if len(older.Turns) != 2 || older.Turns[0].RoundID != "r1" || older.NextBeforeRoundID != "" {
		t.Fatalf("unexpected older page: %+v", older)
	}

	around := PaginateConversationTurns(turns, 1, "", "r3", false)
	if len(around.Turns) != 3 || around.Turns[0].RoundID != "r2" {
		t.Fatalf("unexpected around page: %+v", around)
	}

	desc := PaginateConversationTurns(turns, 2, "", "", true)
	if desc.Turns[0].RoundID != "r4" {
		t.Fatalf("unexpected desc order: %+v", desc.Turns)
	}
}

func TestBuildConversationTurnIndex(t *testing.T) {
	userMessage := protocol.ConversationMessage{Content: "  hello world  "}
	turns := []protocol.ConversationTurn{{
		RoundID:     "round_1",
		Status:      "finished",
		CreatedAt:   1,
		UpdatedAt:   2,
		UserMessage: &userMessage,
		AgentSlots:  []protocol.AgentTurnSlot{{AgentID: "agent_a"}},
		IsLoaded:    true,
	}}
	items := BuildConversationTurnIndex(turns)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.UserPreview != "hello world" || len(item.AgentIDs) != 1 || !item.Loaded {
		t.Fatalf("unexpected index item: %+v", item)
	}
}
