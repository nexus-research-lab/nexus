package realtime

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
)

func TestExternalInputForwardsOnlySuccessfulFinalReply(t *testing.T) {
	var sent []protocol.Message
	e := &slotExecution{ctx: t.Context(), round: &activeRoomRound{RootRoundID: "root"}, slot: &activeRoomSlot{AgentID: "amy", RuntimeSessionKey: "original"}, service: &Service{}}
	e.slot.setDeliveryMetadata(protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone}, "", "")
	e.service.externalReply = func(_ context.Context, root, agent, session string, m protocol.Message) error {
		if root != "root" || agent != "amy" || session != "original" {
			t.Fatal("lost original identity")
		}
		sent = append(sent, m)
		return nil
	}
	commentary := protocol.Message{"role": "assistant", "is_complete": true, "content": "正在查找微信回传渠道"}
	if err := e.emitEvent(protocol.EventMessage{EventType: protocol.EventTypeMessage, Data: commentary}); err != nil {
		t.Fatal(err)
	}
	for _, result := range []exec.RoundExecutionResult{{TerminalStatus: "error"}, {TerminalStatus: "interrupted"}, {TerminalStatus: "finished"}} {
		if err := e.deliverExternalCompletion(result, commentary); err != nil {
			t.Fatal(err)
		}
	}
	success := exec.RoundExecutionResult{TerminalStatus: "finished", CompletedByAssistant: true}
	if err := e.deliverExternalCompletion(success, protocol.Message{"role": "assistant", "content": "<nexus_room_no_reply/>"}); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 0 {
		t.Fatalf("forwarded intermediate or failed output: %v", sent)
	}
	final := protocol.Message{"role": "assistant", "is_complete": true, "content": "之前调研了模型发布情况"}
	if err := e.deliverExternalCompletion(success, final); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0]["content"] != final["content"] {
		t.Fatalf("final reply = %v", sent)
	}
}
