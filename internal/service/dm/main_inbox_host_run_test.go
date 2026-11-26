package dm

import (
	"context"
	"fmt"
	"testing"

	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (f mainInboxFixture) reply(id string, text string) {
	f.client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: f.client.sessionID,
		UUID:      "assistant-" + id,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-" + id,
				Model:      "sonnet",
				StopReason: "end_turn",
				Content:    []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: text}},
			},
		},
	}
}

func (f mainInboxFixture) history(t *testing.T) []protocol.Message {
	t.Helper()
	ctx := context.Background()
	target, _, err := f.service.resolveMainInboxTarget(ctx, f.sessionKey, "")
	if err != nil {
		t.Fatalf("解析主窗口失败: %v", err)
	}
	session, err := f.service.ensureSession(ctx, target.agent, protocol.ParseSessionKey(f.sessionKey), f.sessionKey)
	if err != nil {
		t.Fatalf("读取会话失败: %v", err)
	}
	rows, err := f.service.History.ForOwner(target.owner()).ReadMessages(target.agent.WorkspacePath, session, nil)
	if err != nil {
		t.Fatalf("读取历史失败: %v", err)
	}
	return rows
}

func isHostRunOutcome(data map[string]any, outcome string) bool {
	metadata, _ := data["metadata"].(map[string]any)
	return data["role"] == "system" && metadata["subtype"] == messageutil.SystemMessageSubtypeHostRunOutcome && metadata["outcome"] == outcome
}

func rolesOf(rows []protocol.Message) []string {
	roles := make([]string, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, protocol.MessageRole(row))
	}
	return roles
}

func TestHostRunQuietLeavesOnlyOutcomeRecord(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-run-quiet")
	f.appendTask(t, "run-1")
	f.waitPrompts(t, 1)
	f.reply("quiet", HostNoReplyMarker)
	f.finishRound("quiet")

	events := collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && isHostRunOutcome(event.Data, "quiet")
	})
	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage && event.Data["role"] == "assistant" {
			t.Fatalf("宿主 run 的过程不应推送: %+v", event.Data)
		}
	}
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")

	rows := f.history(t)
	if roles := rolesOf(rows); len(roles) != 2 || roles[0] != "user" || !isHostRunOutcome(rows[1], "quiet") {
		t.Fatalf("静默的宿主 run 只应留下分隔条与结局: %v", roles)
	}
}

func TestHostRunReplyCommitsOneAssistant(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-run-reply")
	f.appendTask(t, "run-1")
	f.waitPrompts(t, 1)
	f.reply("reply", "今天有 3 个提交")
	f.finishRound("reply")

	collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["role"] == "assistant" &&
			messageutil.ExtractAssistantDisplayText(event.Data) == "今天有 3 个提交"
	})
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")

	assistants := 0
	for _, row := range f.history(t) {
		if isHostRunOutcome(row, "quiet") {
			t.Fatalf("有回复的宿主 run 不应记沉默: %+v", row)
		}
		if protocol.MessageRole(row) == "assistant" {
			assistants++
			if text := messageutil.ExtractAssistantDisplayText(row); text != "今天有 3 个提交" {
				t.Fatalf("历史回复 = %q", text)
			}
		}
	}
	if assistants != 1 {
		t.Fatalf("宿主 run 的回复应恰好一条，实际 %d", assistants)
	}
}

func TestHostRunDefersUserInputUntilDone(t *testing.T) {
	for _, runningInput := range []bool{false, true} {
		t.Run(fmt.Sprintf("running_input=%v", runningInput), func(t *testing.T) {
			f := newMainWindowHostFixture(t, "host-run-defer")
			f.client.runningInput = runningInput
			f.appendTask(t, "run-1")
			f.waitPrompts(t, 1)
			f.send(t, "第二件事", "client-2")
			if open := f.waitOpen(t, 1); open[0].Status != inboxstore.StatusPending {
				t.Fatalf("宿主 run 期间的用户输入应留在 inbox: %+v", open)
			}
			if outputs := f.runSafePoint(t); len(outputs) != 0 {
				t.Fatalf("宿主 run 不应在安全点注入用户输入: %+v", outputs)
			}
			f.client.mu.Lock()
			sent := len(f.client.runningInputs)
			f.client.mu.Unlock()
			if sent != 0 {
				t.Fatalf("宿主 run 不应接收运行中输入: %d", sent)
			}
			f.finishRound("defer-1")
			if prompts := f.waitPrompts(t, 2); prompts[1] != "第二件事" {
				t.Fatalf("用户输入应在宿主 run 收口后起新一轮: %q", prompts[1])
			}
			f.finishRound("defer-2")
		})
	}
}
