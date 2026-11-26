package dm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

type fakeMainWindowRooms struct{ conversationID string }

func (r fakeMainWindowRooms) EnsureDirectRoom(context.Context, string) (*protocol.ConversationContextAggregate, error) {
	return &protocol.ConversationContextAggregate{Conversation: protocol.ConversationRecord{ID: r.conversationID}}, nil
}

func newMainWindowHostFixture(t *testing.T, ref string) mainInboxFixture {
	t.Helper()
	f := newMainInboxFixture(t, ref)
	f.service.SetMainWindowRooms(fakeMainWindowRooms{conversationID: ref})
	return f
}

func (f mainInboxFixture) appendTask(t *testing.T, runID string) {
	t.Helper()
	sessionKey, err := f.service.AppendMainWindowTask(context.Background(), "nexus", MainWindowTask{
		JobID:       "job-1",
		RunID:       runID,
		Name:        "日报",
		Instruction: "汇总今天的提交",
		DedupeKey:   "scheduled_task:job-1:" + runID,
	})
	if err != nil {
		t.Fatalf("写入主窗口任务失败: %v", err)
	}
	if sessionKey != f.sessionKey {
		t.Fatalf("session key = %q, want %q", sessionKey, f.sessionKey)
	}
}

func isScheduledTaskRow(data map[string]any) bool {
	metadata, _ := data["metadata"].(map[string]any)
	return data["role"] == "user" && data["is_synthetic"] == true &&
		metadata["input_kind"] == scheduledTaskInputKind && metadata["task_name"] == "日报" && metadata["run_id"] != ""
}

func TestMainWindowTaskStartsOwnRoundWhenIdle(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-idle")
	f.appendTask(t, "run-1")
	prompts := f.waitPrompts(t, 1)
	if !strings.Contains(prompts[0], "<scheduled-task name=\"日报\">") || !strings.Contains(prompts[0], "汇总今天的提交") {
		t.Fatalf("模型输入应标明来自定时任务: %q", prompts[0])
	}
	rows := collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && isScheduledTaskRow(event.Data)
	})
	if len(rows) == 0 {
		t.Fatal("时间线应收到定时任务触发行")
	}
	f.waitOpen(t, 0)
	f.finishRound("host-idle")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")
}

func TestMainWindowTaskInjectsAtSafePointWhileRunning(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-running")
	f.send(t, "先查一下项目结构", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.waitOpen(t, 0)

	f.appendTask(t, "run-1")
	f.appendTask(t, "run-1")
	if pending := f.waitOpen(t, 1); pending[0].Kind != inboxstore.KindScheduledTask {
		t.Fatalf("同一次到点只应留一条宿主事件: %+v", pending)
	}
	outputs := f.runSafePoint(t)
	if len(outputs) != 1 || !strings.Contains(outputs[0].SpecificOutput.AdditionalContext, "<scheduled-task name=\"日报\">") {
		t.Fatalf("运行中到点的任务应在安全点注入: %+v", outputs)
	}
	outputs[0].OnApplied(sdkhook.AppliedAck{})
	f.waitOpen(t, 0)
	collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && isScheduledTaskRow(event.Data)
	})

	f.finishRound("host-running")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "finished")
	time.Sleep(50 * time.Millisecond)
	if prompts := f.prompts(); len(prompts) != 1 {
		t.Fatalf("已注入的任务不应再起新一轮: %+v", prompts)
	}
}

func TestMainWindowTaskRunsAloneAndStaysOutOfQueue(t *testing.T) {
	f := newMainWindowHostFixture(t, "host-queue")
	f.send(t, "第一件事", "client-1")
	waitForEvent(t, f.sender.events, protocol.EventTypeRoundStatus, "running")
	f.appendTask(t, "run-1")
	f.send(t, "第二件事", "client-2")
	f.waitOpen(t, 2)
	// 待发送列表只列用户输入：宿主事件不是用户写的，也不可撤回。
	collectEventsUntil(t, f.sender.events, func(event protocol.EventMessage) bool {
		items, _ := event.Data["items"].([]protocol.InputQueueItem)
		return event.EventType == protocol.EventTypeInputQueue && len(items) == 1 && items[0].Content == "第二件事"
	})
	f.finishRound("host-queue-1")
	// 任务独占一轮，用户输入随后单独起轮。
	prompts := f.waitPrompts(t, 2)
	if !strings.Contains(prompts[1], "<scheduled-task") || strings.Contains(prompts[1], "第二件事") {
		t.Fatalf("宿主事件应独占一轮: %q", prompts[1])
	}
	f.finishRound("host-queue-2")
	prompts = f.waitPrompts(t, 3)
	if prompts[2] != "第二件事" {
		t.Fatalf("用户输入应在任务轮之后执行: %q", prompts[2])
	}
	f.finishRound("host-queue-3")
}

func TestRoundBatchKeepsHostEventsAlone(t *testing.T) {
	user := func(id string) inboxstore.Event {
		return inboxstore.Event{EventID: id, Kind: inboxstore.KindUserMessage, Payload: inboxstore.Payload{Content: id}}
	}
	host := func(id string) inboxstore.Event {
		return inboxstore.Event{EventID: id, Kind: inboxstore.KindScheduledTask, Payload: inboxstore.Payload{Content: "/compact"}}
	}
	if got := strings.Join(eventIDs(roundBatch([]inboxstore.Event{host("t"), user("a")})), ","); got != "t" {
		t.Fatalf("宿主事件应独占一轮: %s", got)
	}
	if got := strings.Join(eventIDs(roundBatch([]inboxstore.Event{user("a"), host("t")})), ","); got != "a" {
		t.Fatalf("用户输入不应与宿主事件合并: %s", got)
	}
	// 宿主事件的指令文本不是斜杠命令，安全点可照常注入。
	if got := strings.Join(eventIDs(guidanceBatch([]inboxstore.Event{user("a"), host("t")})), ","); got != "a,t" {
		t.Fatalf("安全点应注入宿主事件: %s", got)
	}
}
