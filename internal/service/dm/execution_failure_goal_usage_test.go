package dm

import (
	"errors"
	"testing"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestFailRoundKeepsProviderUsageReturnedBeforeLocalFailure(t *testing.T) {
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
			cfg := newDMTestConfig(t)
			goalProvider := &fakeGoalContextProvider{}
			sessionKey := "agent:nexus:ws:dm:terminal-local-failure"
			roundID := "round-terminal-local-failure"
			agentID := "agent-terminal-local-failure"
			service := NewService(
				cfg,
				nil,
				runtimectx.NewManager(),
				permissionctx.NewContext(),
			)
			service.goals = goalProvider
			runner := &roundRunner{
				service:       service,
				workspacePath: cfg.WorkspacePath,
				session: protocol.Session{
					SessionKey: sessionKey,
					AgentID:    agentID,
				},
				agent: &protocol.Agent{
					AgentID:       agentID,
					WorkspacePath: cfg.WorkspacePath,
				},
				sessionKey:       sessionKey,
				roundID:          roundID,
				agentRoundID:     roundID,
				client:           newFakeDMClient(),
				mapper:           dmdomain.NewMessageMapper(sessionKey, agentID, roundID, roundID, "user-terminal-local-failure"),
				goalIDForUsage:   "goal-terminal-local-failure",
				goalUsage:        goalsvc.NewRuntimeUsageAccumulator(true),
				goalUsageStarted: time.Now(),
			}
			runner.rememberGoalAssistantMessage(protocol.Message{
				"message_id": "assistant-terminal-local-failure",
				"role":       "assistant",
				"usage": map[string]any{
					"input_tokens":  int64(70),
					"output_tokens": int64(30),
					"total_tokens":  int64(100),
				},
			})

			runner.failRound(
				exec.RoundExecutionResult{Usage: testCase.usage},
				errors.New("persist terminal event"),
			)

			usages := goalProvider.recordedUsage()
			if len(usages) != testCase.wantWrites {
				t.Fatalf("failure-settled usage = %#v, want %d writes", usages, testCase.wantWrites)
			}
			if len(usages) > 0 && usages[0].ActualTokens() != testCase.wantActual {
				t.Fatalf("failure-settled usage = %#v, want actual %d", usages, testCase.wantActual)
			}
			if runner.goalUsage.Active() {
				t.Fatal("Goal usage remains active after failure settlement")
			}
		})
	}
}
