package message

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestEventMapperProjectsCompactRuntimeStatusLifecycle(t *testing.T) {
	mapper := NewEventMapper(EventMapperOptions{
		Context: MessageContext{
			SessionKey: "agent:nexus:ws:dm:test",
			AgentID:    "nexus",
			RoundID:    "round-compact",
		},
	})

	statuses := map[string]any{
		"compacting": protocol.RuntimeStatusCompacting,
		"":           nil,
	}
	for status, want := range statuses {
		t.Run(status, func(t *testing.T) {
			result, err := mapper.Map(sdkprotocol.ReceivedMessage{
				Type:    sdkprotocol.MessageTypeSystem,
				Subtype: "status",
				System: &sdkprotocol.SystemMessage{
					Subtype: "status",
					Status:  &sdkprotocol.StatusSystemMessage{Status: status},
				},
			})
			if err != nil {
				t.Fatalf("Map(status=%q) error = %v", status, err)
			}
			if len(result.Events) != 1 || result.Events[0].EventType != protocol.EventTypeRuntimeStatus {
				t.Fatalf("events = %+v", result.Events)
			}
			if result.Events[0].Data["status"] != want {
				t.Fatalf("data = %+v, want status %#v", result.Events[0].Data, want)
			}
		})
	}
}

func TestEventMapperInvalidatesOnlySubagentTaskChanges(t *testing.T) {
	mapper := NewEventMapper(EventMapperOptions{Context: MessageContext{
		SessionKey:     "agent:nexus:ws:room:conversation-1",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		AgentID:        "nexus",
		RoundID:        "round-subagent",
	}})

	started, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeTaskStarted,
		TaskStarted: &sdkprotocol.TaskStartedMessage{
			TaskID:    "task-agent",
			ToolUseID: "tool-agent",
			TaskType:  "local_agent",
		},
	})
	if err != nil {
		t.Fatalf("Map(task_started) error = %v", err)
	}
	event := findMappedEvent(started.Events, protocol.EventTypeSubagentTaskChanged)
	if event == nil {
		t.Fatalf("task_started events = %+v", started.Events)
	}
	if event.RoomID != "room-1" || event.ConversationID != "conversation-1" || event.AgentID != "nexus" {
		t.Fatalf("失效事件作用域不正确: %+v", event)
	}
	taskIDs, ok := event.Data["task_ids"].([]string)
	if !ok || len(taskIDs) != 1 || taskIDs[0] != "task-agent" {
		t.Fatalf("task_ids = %#v", event.Data["task_ids"])
	}
	unchanged, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeTaskStarted,
		TaskStarted: &sdkprotocol.TaskStartedMessage{
			TaskID:    "task-agent",
			ToolUseID: "tool-agent",
			TaskType:  "local_agent",
		},
	})
	if err != nil {
		t.Fatalf("Map(unchanged task_started) error = %v", err)
	}
	if event := findMappedEvent(unchanged.Events, protocol.EventTypeSubagentTaskChanged); event != nil {
		t.Fatalf("未变化的任务快照不应重复失效: %+v", event)
	}

	completed, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeTaskNotification,
		TaskNotification: &sdkprotocol.TaskNotificationMessage{
			TaskID: "task-agent",
			Status: "completed",
		},
	})
	if err != nil || findMappedEvent(completed.Events, protocol.EventTypeSubagentTaskChanged) == nil {
		t.Fatalf("已识别任务的终态未失效: result=%+v error=%v", completed, err)
	}

	shell, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeTaskStarted,
		TaskStarted: &sdkprotocol.TaskStartedMessage{
			TaskID:   "task-shell",
			TaskType: "local_shell",
		},
	})
	if err != nil {
		t.Fatalf("Map(local_shell) error = %v", err)
	}
	if event := findMappedEvent(shell.Events, protocol.EventTypeSubagentTaskChanged); event != nil {
		t.Fatalf("local_shell 不应触发子智能体失效: %+v", event)
	}
}

func findMappedEvent(events []protocol.EventMessage, eventType protocol.EventType) *protocol.EventMessage {
	for index := range events {
		if events[index].EventType == eventType {
			return &events[index]
		}
	}
	return nil
}

func TestEventMapperDecoratesDurableAndProjectedMessages(t *testing.T) {
	mapper := NewEventMapper(EventMapperOptions{
		Context: MessageContext{
			SessionKey: "agent:nexus:ws:dm:test",
			AgentID:    "nexus",
			RoundID:    "round-decoration",
		},
	})
	decoratedRoles := make([]string, 0, 3)
	mapper.SetMessageDecorator(func(message protocol.Message) {
		decoratedRoles = append(decoratedRoles, protocol.MessageRole(message))
		message["decorated"] = true
	})

	assistant, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-decoration",
			StopReason: "end_turn",
			Content:    []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{Text: "完成"}},
		}},
	})
	if err != nil {
		t.Fatalf("assistant Map() error = %v", err)
	}
	if len(assistant.DurableMessages) != 1 || assistant.DurableMessages[0]["decorated"] != true {
		t.Fatalf("assistant durable 未装饰: %+v", assistant.DurableMessages)
	}
	if len(assistant.Events) != 1 || assistant.Events[0].DeliveryMode != protocol.DeliveryModeDurable {
		t.Fatalf("assistant durable event 不正确: %+v", assistant.Events)
	}

	result, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
			Result:  "完成",
		},
	})
	if err != nil {
		t.Fatalf("result Map() error = %v", err)
	}
	if len(result.DurableMessages) != 1 || result.DurableMessages[0]["decorated"] != true {
		t.Fatalf("result durable 未装饰: %+v", result.DurableMessages)
	}
	if len(result.Events) != 1 || result.Events[0].Data["decorated"] != true {
		t.Fatalf("result assistant 投影未装饰: %+v", result.Events)
	}
	wantRoles := []string{"assistant", "result", "assistant"}
	if len(decoratedRoles) != len(wantRoles) {
		t.Fatalf("装饰顺序 = %+v, want %+v", decoratedRoles, wantRoles)
	}
	for index, role := range wantRoles {
		if decoratedRoles[index] != role {
			t.Fatalf("装饰顺序 = %+v, want %+v", decoratedRoles, wantRoles)
		}
	}
	if mapper.LastAssistantMessage()["decorated"] != true {
		t.Fatalf("终态 assistant 快照未保留装饰字段: %+v", mapper.LastAssistantMessage())
	}
}

func TestEventMapperDecoratesExplicitResultProjection(t *testing.T) {
	mapper := NewEventMapper(EventMapperOptions{
		Context: MessageContext{RoundID: "round-explicit-projection"},
	})
	mapper.SetMessageDecorator(func(message protocol.Message) {
		message["decorated"] = true
	})

	projected := mapper.ProjectResultMessage(protocol.Message{
		"message_id": "result-explicit-projection",
		"round_id":   "round-explicit-projection",
		"role":       "result",
		"subtype":    "error",
		"is_error":   true,
		"result":     "runtime failed",
	})
	if len(projected) == 0 || projected["decorated"] != true {
		t.Fatalf("显式 result 投影未装饰: %+v", projected)
	}
	if mapper.LastAssistantMessage()["decorated"] != true {
		t.Fatalf("显式投影未更新终态快照: %+v", mapper.LastAssistantMessage())
	}
}

func TestEventMapperKeepsEmptySuccessfulResultDurableWithoutEvent(t *testing.T) {
	mapper := NewEventMapper(EventMapperOptions{
		Context: MessageContext{RoundID: "round-empty-result"},
	})

	result, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{Subtype: "success"},
	})
	if err != nil {
		t.Fatalf("Map() error = %v", err)
	}
	if len(result.DurableMessages) != 1 || len(result.Events) != 0 {
		t.Fatalf("空 success result 映射不正确: %+v", result)
	}
}

func TestEventMapperFinalizesInterruptedStreamContent(t *testing.T) {
	mapper := NewEventMapper(EventMapperOptions{
		Context: MessageContext{
			AgentID:      "agent-1",
			RoundID:      "round-interrupted",
			AgentRoundID: "agent-round-interrupted",
		},
	})
	mapper.SetMessageDecorator(func(message protocol.Message) {
		message["decorated"] = true
	})
	stream := []sdkprotocol.ReceivedMessage{
		{
			Type: sdkprotocol.MessageTypeStreamEvent,
			Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
				"type":    "message_start",
				"message": map[string]any{"id": "assistant-interrupted"},
			}},
		},
		{
			Type: sdkprotocol.MessageTypeStreamEvent,
			Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type": "thinking", "thinking": "先分析",
				},
			}},
		},
		{
			Type: sdkprotocol.MessageTypeStreamEvent,
			Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type": "thinking_delta", "thinking": "再决定",
				},
			}},
		},
	}
	for _, incoming := range stream {
		if _, err := mapper.Map(incoming); err != nil {
			t.Fatalf("Map() error = %v", err)
		}
	}

	partial := mapper.FinalizeInterruptedAssistant()
	if partial["is_complete"] != true || partial["stop_reason"] != "cancelled" {
		t.Fatalf("中断快照未收口: %+v", partial)
	}
	if partial["is_interrupted_partial"] != true {
		t.Fatalf("中断快照必须进入 overlay: %+v", partial)
	}
	blocks := partial["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["thinking"] != "先分析再决定" {
		t.Fatalf("中断快照未保留流式内容: %+v", partial)
	}
	if partial["decorated"] != true || mapper.LastAssistantMessage()["decorated"] != true {
		t.Fatalf("中断快照未经过场景装饰: %+v", partial)
	}
	if duplicate := mapper.FinalizeInterruptedAssistant(); duplicate != nil {
		t.Fatalf("重复收口不应再次生成快照: %+v", duplicate)
	}
}
