package realtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func newFailureSettlementAuthority(
	roomID string,
	conversationID string,
	agentID string,
) (*authorityFenceRoomStore, *protocol.ConversationContextAggregate) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID:             roomID,
			RoomType:       protocol.RoomTypeGroup,
			AuthorityEpoch: 1,
		},
		Conversation: protocol.ConversationRecord{
			ID:     conversationID,
			RoomID: roomID,
		},
		Members: []protocol.MemberRecord{{
			MemberType:    protocol.MemberTypeAgent,
			MemberAgentID: agentID,
		}},
	}
	return &authorityFenceRoomStore{contextValue: contextValue}, contextValue
}

func TestHandleSlotFailureFinalizesRememberedAssistantAfterDurablePersistenceFailure(t *testing.T) {
	blockedRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("block room history"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_STATE_ROOT", blockedRoot)

	goalProvider := &fakeRoomGoalContextProvider{}
	rooms, authority := newFailureSettlementAuthority(
		"room-persist-failure",
		"conversation-persist-failure",
		"agent-persist-failure",
	)
	service := &Service{
		rooms:       rooms,
		goals:       goalProvider,
		permission:  permissionctx.NewContext(),
		roomHistory: workspacestore.NewRoomHistoryStore(blockedRoot),
	}
	roundValue := &activeRoomRound{
		SessionKey:     "room:group:persist-failure",
		RoomID:         "room-persist-failure",
		ConversationID: "conversation-persist-failure",
		RootRoundID:    "round-persist-failure",
		Context:        cloneAuthorityFenceContext(authority),
		AuthorityEpoch: authority.Room.AuthorityEpoch,
	}
	slot := &activeRoomSlot{
		AgentID:           "agent-persist-failure",
		AgentRoundID:      "round-persist-failure:agent",
		MsgID:             "message-persist-failure",
		RuntimeSessionKey: "agent:persist-failure:ws:group:conversation",
	}
	slot.setGoalBinding(roundValue.SessionKey, "goal-persist-failure")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	execution := &slotExecution{
		service: service,
		ctx:     context.Background(),
		round:   roundValue,
		slot:    slot,
	}
	assistant := protocol.Message{
		"message_id": "assistant-persist-failure",
		"role":       "assistant",
		"usage": map[string]any{
			"input_tokens":  int64(8),
			"output_tokens": int64(3),
			"total_tokens":  int64(11),
		},
	}

	persistErr := execution.handleDurableMessage(assistant)
	if persistErr == nil {
		t.Fatal("handleDurableMessage() error = nil, want room history persistence failure")
	}
	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("usage before failure settlement = %#v, want none", usages)
	}

	service.handleSlotFailure(context.Background(), roundValue, slot, nil, exec.RoundExecutionResult{}, persistErr)

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 ||
		usages[0].InputTokens != 8 ||
		usages[0].OutputTokens != 3 ||
		usages[0].ActualTokens() != 11 {
		t.Fatalf("failure-settled usage = %#v, want exact remembered assistant 8/3", usages)
	}
	if slot.goalUsageActive() {
		t.Fatal("Goal usage remains active after failure settlement")
	}
}

func TestHandleSlotFailureKeepsProviderUsageReturnedBeforeLocalFailure(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		usage      sdkprotocol.TokenUsage
		wantWrites int
		wantActual int64
	}{
		{
			name: "nonzero exact",
			usage: sdkprotocol.TokenUsage{
				InputTokens:  10,
				OutputTokens: 5,
				TotalTokens:  15,
			},
			wantWrites: 1,
			wantActual: 15,
		},
		{
			name: "explicit zero overrides assistant estimate",
			usage: sdkprotocol.TokenUsage{
				Raw: map[string]any{"total_tokens": int64(0)},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			blockedRoot := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(blockedRoot, []byte("block room history"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("NEXUS_STATE_ROOT", blockedRoot)

			goalProvider := &fakeRoomGoalContextProvider{}
			rooms, authority := newFailureSettlementAuthority(
				"room-terminal-local-failure",
				"conversation-terminal-local-failure",
				"agent-terminal-local-failure",
			)
			service := &Service{
				rooms:       rooms,
				goals:       goalProvider,
				permission:  permissionctx.NewContext(),
				roomHistory: workspacestore.NewRoomHistoryStore(blockedRoot),
			}
			roundValue := &activeRoomRound{
				SessionKey:     "room:group:terminal-local-failure",
				RoomID:         "room-terminal-local-failure",
				ConversationID: "conversation-terminal-local-failure",
				RootRoundID:    "round-terminal-local-failure",
				Context:        cloneAuthorityFenceContext(authority),
				AuthorityEpoch: authority.Room.AuthorityEpoch,
			}
			slot := &activeRoomSlot{
				AgentID:           "agent-terminal-local-failure",
				AgentRoundID:      "round-terminal-local-failure:agent",
				MsgID:             "message-terminal-local-failure",
				RuntimeSessionKey: "agent:terminal-local-failure:ws:group:conversation",
			}
			slot.setGoalBinding(roundValue.SessionKey, "goal-terminal-local-failure")
			slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
			slot.rememberGoalAssistantMessage(protocol.Message{
				"message_id": "assistant-terminal-local-failure",
				"role":       "assistant",
				"usage": map[string]any{
					"input_tokens":  int64(70),
					"output_tokens": int64(30),
					"total_tokens":  int64(100),
				},
			})

			service.handleSlotFailure(
				context.Background(),
				roundValue,
				slot,
				nil,
				exec.RoundExecutionResult{Usage: testCase.usage},
				os.ErrPermission,
			)

			usages := goalProvider.recordedUsage()
			if len(usages) != testCase.wantWrites {
				t.Fatalf("failure-settled usage = %#v, want %d writes", usages, testCase.wantWrites)
			}
			if len(usages) > 0 && usages[0].ActualTokens() != testCase.wantActual {
				t.Fatalf("failure-settled usage = %#v, want actual %d", usages, testCase.wantActual)
			}
			if slot.goalUsageActive() {
				t.Fatal("Goal usage remains active after failure settlement")
			}
		})
	}
}
