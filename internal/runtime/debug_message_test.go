package runtime

import (
	"encoding/json"
	"strings"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestBuildSDKMessageLogSummaryForStreamEvent(t *testing.T) {
	summary := BuildSDKMessageLogSummary(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_delta",
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": "正在分析天气问题",
				},
			},
		},
	})

	if summary != "stream content_block_delta(thinking_delta)" {
		t.Fatalf("stream 摘要不符合预期: %s", summary)
	}
	if strings.Contains(summary, "正在分析天气问题") {
		t.Fatalf("stream 摘要不应包含 thinking 正文: %s", summary)
	}
}

func TestBuildSDKMessageLogSummaryForAssistantSnapshot(t *testing.T) {
	summary := BuildSDKMessageLogSummary(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "msg-assistant",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先分析"},
					sdkprotocol.TextBlock{Text: "再回答"},
				},
			},
		},
	})

	if summary != "assistant snapshot(thinking,text)" {
		t.Fatalf("assistant 摘要不符合预期: %s", summary)
	}
	if strings.Contains(summary, "先分析") || strings.Contains(summary, "再回答") {
		t.Fatalf("assistant 摘要不应包含正文: %s", summary)
	}
}

func TestBuildSDKMessageLogSummaryRedactsToolInputDelta(t *testing.T) {
	summary := BuildSDKMessageLogSummary(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_delta",
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": `{"command":"ln -s ../../.agents/skills/demo /Users/me/.nexus/workspace/.claude/skills/demo"}`,
				},
			},
		},
	})

	if summary != "stream content_block_delta(input_json_delta)" {
		t.Fatalf("tool input delta 摘要不符合预期: %s", summary)
	}
	if strings.Contains(summary, "command") || strings.Contains(summary, "/Users/me") {
		t.Fatalf("tool input delta 摘要不应包含工具参数: %s", summary)
	}
}

func TestBuildSDKMessageLogSummaryKeepsToolNameOnly(t *testing.T) {
	summary := BuildSDKMessageLogSummary(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_start",
				"content_block": map[string]any{
					"type":  "tool_use",
					"name":  "Bash",
					"input": map[string]any{"command": "cat SECRET.txt"},
				},
			},
		},
	})

	if summary != `stream content_block_start(tool_use) "Bash"` {
		t.Fatalf("tool_use start 摘要不符合预期: %s", summary)
	}
	if strings.Contains(summary, "SECRET") || strings.Contains(summary, "cat ") {
		t.Fatalf("tool_use start 摘要不应包含工具输入: %s", summary)
	}
}

func TestBuildSDKMessageLogSummaryForToolResult(t *testing.T) {
	summary := BuildSDKMessageLogSummary(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "msg-tool-result",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "toolu_123",
						Content:   json.RawMessage(`"SECRET result"`),
						IsError:   true,
					},
				},
			},
		},
	})

	if summary != "user snapshot(tool_result)" {
		t.Fatalf("tool_result 摘要不符合预期: %s", summary)
	}
	if strings.Contains(summary, "SECRET") || strings.Contains(summary, "toolu_123") {
		t.Fatalf("tool_result 摘要不应包含结果正文或标识: %s", summary)
	}
}

func TestBuildSDKMessageLogFieldsIncludesSummary(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
			Result:  "查询完成",
		},
	})

	if len(fields) < 2 {
		t.Fatalf("日志字段数量异常: %+v", fields)
	}
	if key, ok := fields[0].(string); !ok || key != "sdk_summary" {
		t.Fatalf("首个字段应为 sdk_summary: %+v", fields)
	}
	if value, ok := fields[1].(string); !ok || value != "result success" {
		t.Fatalf("sdk_summary 值异常: %+v", fields)
	}
}

func TestBuildSDKMessageLogFieldsIncludesToolResultCounts(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "toolu_123",
						Content:   json.RawMessage(`"SECRET result"`),
						IsError:   true,
					},
				},
			},
		},
	})

	if !hasLogField(fields, "tool_results", 1) {
		t.Fatalf("缺少 tool_result 数量: %+v", fields)
	}
	if !hasLogField(fields, "tool_errors", 1) {
		t.Fatalf("缺少 tool_result 错误数量: %+v", fields)
	}
	for _, field := range fields {
		if value, ok := field.(string); ok && strings.Contains(value, "SECRET") {
			t.Fatalf("tool_result 不应输出结果正文: %+v", fields)
		}
	}
	if hasLogFieldKey(fields, "tool_use_id") {
		t.Fatalf("tool_result 不应输出工具标识字段: %+v", fields)
	}
}

func TestBuildSDKMessageLogFieldsIncludesStreamTextDelta(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "session-123",
		UUID:      "uuid-456",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type": "text_delta",
					"text": "正在输出给用户看的内容",
				},
			},
		},
	})

	if !hasLogField(fields, "delta", "正在输出给用户看的内容") {
		t.Fatalf("缺少 stream 文本增量: %+v", fields)
	}
	if !hasLogField(fields, "stream_event", "content_block_delta") {
		t.Fatalf("缺少 stream event 类型: %+v", fields)
	}
	if !hasLogField(fields, "stream_index", 0) {
		t.Fatalf("缺少 stream block index: %+v", fields)
	}
	if !hasLogField(fields, "stream_delta", "text_delta") {
		t.Fatalf("缺少 stream delta 类型: %+v", fields)
	}
	if hasLogFieldKey(fields, "sdk_session_id") ||
		hasLogFieldKey(fields, "sdk_message_uuid") ||
		hasLogFieldKey(fields, "stream_event_type") ||
		hasLogFieldKey(fields, "stream_delta_type") {
		t.Fatalf("不应输出冗余 SDK 标识字段: %+v", fields)
	}
}

func TestBuildSDKMessageLogFieldsCanHideStreamEvent(t *testing.T) {
	fields := BuildSDKMessageLogFieldsWithOptions(
		sdkprotocol.ReceivedMessage{
			Type: sdkprotocol.MessageTypeStreamEvent,
			Stream: &sdkprotocol.StreamEvent{
				Event: map[string]any{
					"type": "content_block_delta",
					"delta": map[string]any{
						"type":     "thinking_delta",
						"thinking": "不应该出现在日志字段里的思考过程",
					},
				},
			},
		},
		SDKMessageLogOptions{IncludeStreamEvent: false, IncludeSnapshotData: true},
	)

	if len(fields) != 0 {
		t.Fatalf("关闭 StreamEvent 后不应输出任何 StreamEvent 日志字段: %+v", fields)
	}
}

func TestBuildSDKMessageLogSummaryFollowsOfficialStreamFlow(t *testing.T) {
	cases := []struct {
		name    string
		event   map[string]any
		summary string
	}{
		{
			name: "message_start",
			event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"role":  "assistant",
					"model": "claude-sonnet-4-5",
				},
			},
			summary: "stream message_start(assistant)",
		},
		{
			name: "content_block_stop",
			event: map[string]any{
				"type":  "content_block_stop",
				"index": 0,
			},
			summary: "stream content_block_stop",
		},
		{
			name: "message_delta",
			event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "end_turn",
				},
			},
			summary: "stream message_delta(stop_reason=end_turn)",
		},
		{
			name: "message_stop",
			event: map[string]any{
				"type": "message_stop",
			},
			summary: "stream message_stop",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			summary := BuildSDKMessageLogSummary(sdkprotocol.ReceivedMessage{
				Type: sdkprotocol.MessageTypeStreamEvent,
				Stream: &sdkprotocol.StreamEvent{
					Event: tt.event,
				},
			})
			if summary != tt.summary {
				t.Fatalf("stream flow 摘要不符合预期: got=%q want=%q", summary, tt.summary)
			}
		})
	}
}

func TestBuildSDKMessageLogFieldsForMessageDelta(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason":   "tool_use",
					"stop_sequence": "ignored",
				},
			},
		},
	})

	if !hasLogField(fields, "stream_event", "message_delta") {
		t.Fatalf("缺少 message_delta event 类型: %+v", fields)
	}
	if !hasLogField(fields, "stream_stop_reason", "tool_use") {
		t.Fatalf("缺少 stop_reason: %+v", fields)
	}
}

func TestBuildSDKMessageLogFieldsIncludesAssistantSnapshotText(t *testing.T) {
	fullText := strings.Repeat("完整文本", 80) + "最终结尾"
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: fullText},
				},
			},
		},
	})

	if !hasLogField(fields, "assistant_text", fullText) {
		t.Fatalf("assistant snapshot 应输出完整文本: %+v", fields)
	}
}

func TestBuildSDKMessageLogFieldsCanHideAssistantSnapshotText(t *testing.T) {
	fields := BuildSDKMessageLogFieldsWithOptions(
		sdkprotocol.ReceivedMessage{
			Type: sdkprotocol.MessageTypeAssistant,
			Assistant: &sdkprotocol.AssistantMessage{
				Message: sdkprotocol.ConversationEnvelope{
					Content: []sdkprotocol.ContentBlock{
						sdkprotocol.TextBlock{Text: "不应该出现在日志字段里的最终正文"},
					},
				},
			},
		},
		SDKMessageLogOptions{IncludeStreamEvent: true, IncludeSnapshotData: false},
	)

	if !hasLogField(fields, "sdk_summary", "assistant snapshot(text)") {
		t.Fatalf("关闭 snapshot 数据后仍应保留摘要: %+v", fields)
	}
	if hasLogFieldKey(fields, "assistant_text") {
		t.Fatalf("关闭 snapshot 数据后不应输出最终正文: %+v", fields)
	}
	for _, field := range fields {
		if value, ok := field.(string); ok && strings.Contains(value, "不应该出现在日志字段") {
			t.Fatalf("关闭 snapshot 数据后不应输出最终正文: %+v", fields)
		}
	}
}

func TestBuildSDKMessageLogFieldsIncludesThinkingDelta(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_delta",
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": "先判断上下文，再组织答案",
				},
			},
		},
	})

	if !hasLogField(fields, "thinking", "先判断上下文，再组织答案") {
		t.Fatalf("缺少 thinking 增量: %+v", fields)
	}
	if hasLogFieldKey(fields, "stream_event_type") || hasLogFieldKey(fields, "stream_delta_type") {
		t.Fatalf("thinking 日志不应输出冗余 stream 字段: %+v", fields)
	}
}

func TestBuildSDKMessageLogFieldsRedactsInternalToken(t *testing.T) {
	rawToken := "bf9d57b983480d82f932a241d19561a8e01fef796ea33df5b95def8d82e55968"
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_delta",
				"delta": map[string]any{
					"type": "text_delta",
					"text": "NEXUS_ROOM_INTERNAL_TOKEN=" + rawToken + " go run ./cmd/nexusctl",
				},
			},
		},
	})

	for _, field := range fields {
		value, ok := field.(string)
		if !ok {
			continue
		}
		if strings.Contains(value, rawToken) {
			t.Fatalf("内部 token 不应进入日志字段: %+v", fields)
		}
	}
	if !hasLogField(fields, "delta", "NEXUS_ROOM_INTERNAL_TOKEN=[redacted] go run ./cmd/nexusctl") {
		t.Fatalf("内部 token 应被脱敏: %+v", fields)
	}
}

func TestRedactSensitiveTextRedactsInternalTokenForms(t *testing.T) {
	rawToken := "0123456789abcdef0123456789abcdef"
	cases := []string{
		"NEXUS_ROOM_INTERNAL_TOKEN=" + rawToken,
		`{"NEXUS_ROOM_INTERNAL_TOKEN":"` + rawToken + `"}`,
		"X-Nexus-Internal-Token: " + rawToken,
		`{"X-Nexus-Internal-Token":"` + rawToken + `"}`,
	}
	for _, input := range cases {
		output := RedactSensitiveText(input)
		if strings.Contains(output, rawToken) {
			t.Fatalf("敏感内容未脱敏: input=%q output=%q", input, output)
		}
		if !strings.Contains(output, "[redacted]") {
			t.Fatalf("脱敏占位缺失: input=%q output=%q", input, output)
		}
	}
}

func TestBuildSDKMessageLogFieldsKeepsToolNameOnly(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_start",
				"content_block": map[string]any{
					"type":  "tool_use",
					"name":  "Bash",
					"input": map[string]any{"command": "cat SECRET.txt"},
				},
			},
		},
	})

	if !hasLogField(fields, "tool", "Bash") {
		t.Fatalf("缺少 tool 名称: %+v", fields)
	}
	for _, field := range fields {
		if value, ok := field.(string); ok && strings.Contains(value, "SECRET") {
			t.Fatalf("tool start 不应输出工具输入: %+v", fields)
		}
	}
}

func TestBuildSDKMessageLogFieldsIncludesToolProgress(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeToolProgress,
		ToolProgress: &sdkprotocol.ToolProgressMessage{
			ToolUseID:          "toolu_123",
			ToolName:           "Read",
			TaskID:             "task_456",
			ElapsedTimeSeconds: 1.25,
		},
	})

	if !hasLogField(fields, "tool", "Read") {
		t.Fatalf("缺少 tool progress 名称: %+v", fields)
	}
	if !hasLogField(fields, "elapsed_sec", 1.25) {
		t.Fatalf("缺少 tool progress 耗时: %+v", fields)
	}
	if hasLogFieldKey(fields, "tool_use_id") || hasLogFieldKey(fields, "task_id") {
		t.Fatalf("tool progress 不应输出冗余标识字段: %+v", fields)
	}
}

func TestBuildSDKMessageLogFieldsRedactsToolInputDelta(t *testing.T) {
	fields := BuildSDKMessageLogFields(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_delta",
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": `{"command":"cat SECRET.txt"}`,
				},
			},
		},
	})

	for _, field := range fields {
		if value, ok := field.(string); ok && strings.Contains(value, "SECRET") {
			t.Fatalf("tool input delta 不应进入日志字段: %+v", fields)
		}
	}
	if hasLogFieldKey(fields, "stream_text_delta") || hasLogFieldKey(fields, "stream_text") {
		t.Fatalf("tool input delta 不应输出文本字段: %+v", fields)
	}
}

func hasLogField(fields []any, key string, value any) bool {
	for index := 0; index+1 < len(fields); index += 2 {
		if fields[index] == key && fields[index+1] == value {
			return true
		}
	}
	return false
}

func hasLogFieldKey(fields []any, key string) bool {
	for index := 0; index < len(fields); index += 2 {
		if fields[index] == key {
			return true
		}
	}
	return false
}
