package realtime

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestLeasedChatNeverFallsBackToUserQueue(t *testing.T) {
	service := &Service{rounds: newRoomRoundRegistry()}
	contextValue := &protocol.ConversationContextAggregate{Members: []protocol.MemberRecord{{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent", ParticipationPaused: true}}}
	execution := roomChatExecution{service: service, request: ChatRequest{requireImmediateStart: true}, contextValue: contextValue, sessionKey: "room:conversation", conversationID: "conversation", targetAgentIDs: []string{"agent"}}
	// 未装配任何用户队列：错误地退入普通路径将失败或触发 panic。
	if handled, err := execution.routeActiveSlots(); !handled || err == nil {
		t.Fatalf("paused agent queued: %v %v", handled, err)
	}
	contextValue.Members[0].ParticipationPaused = false
	if handled, err := execution.routeActiveSlots(); handled || err != nil {
		t.Fatalf("idle agent rejected: %v %v", handled, err)
	}
	slot := &activeRoomSlot{AgentID: "agent", AgentRoundID: "agent-round"}
	slot.setStatus("running")
	service.rounds.register(&activeRoomRound{SessionKey: "room:conversation", ConversationID: "conversation", RoundID: "busy", Slots: map[string]*activeRoomSlot{"agent": slot}})
	if handled, err := execution.routeActiveSlots(); !handled || err == nil {
		t.Fatalf("busy agent queued: %v %v", handled, err)
	}
}
