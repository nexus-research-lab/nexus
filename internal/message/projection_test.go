// projection_test.go 覆盖消息投影、归约、中断原因、权限审核、唤醒、规范 JSON 与密钥脱敏测试。
package message

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGoalCompletionReceiptOnlyExposesKnownSettlementValues(t *testing.T) {
	seconds := int64(754)
	tests := []struct {
		name       string
		report     *protocol.GoalUsageReport
		wantTime   *int64
		wantTokens *int64
	}{
		{name: "completion only"},
		{
			name: "duration without unavailable tokens",
			report: &protocol.GoalUsageReport{
				Status:          protocol.GoalStatusComplete,
				TimeUsedSeconds: seconds,
			},
			wantTime: &seconds,
		},
		{
			name: "finalized zero is authoritative",
			report: &protocol.GoalUsageReport{
				Status:         protocol.GoalStatusComplete,
				UsageFinalized: true,
			},
			wantTokens: int64TestPointer(0),
		},
		{
			name: "duration and finalized actual tokens",
			report: &protocol.GoalUsageReport{
				Status:          protocol.GoalStatusComplete,
				TimeUsedSeconds: seconds,
				UsageFinalized:  true,
				Usage: protocol.GoalUsage{
					ActualTotalTokens: 62762,
					ActualTotalKnown:  true,
				},
			},
			wantTime:   &seconds,
			wantTokens: int64TestPointer(62762),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receipt := BuildGoalCompletionReceipt("goal-1", "round-1", test.report)
			if !optionalInt64Equal(receipt.TimeUsedSeconds, test.wantTime) ||
				!optionalInt64Equal(receipt.ActualTokens, test.wantTokens) {
				t.Fatalf("receipt = %+v, want time=%v tokens=%v", receipt, test.wantTime, test.wantTokens)
			}
		})
	}
}

func TestAttachGoalCompletionReceiptRequiresExactRoundIdentity(t *testing.T) {
	assistant := protocol.Message{
		"message_id":     "assistant-final",
		"role":           "assistant",
		"round_id":       "root-round",
		"agent_round_id": "agent-round",
	}
	receipt := protocol.GoalCompletionReceipt{GoalID: "goal-1", RoundID: "agent-round"}
	attached, ok := AttachGoalCompletionReceipt(assistant, receipt)
	if !ok || attached[protocol.GoalCompletionReceiptField] == nil {
		t.Fatalf("exact receipt was not attached: %+v", attached)
	}
	receipt.RoundID = "other-round"
	if attached, ok := AttachGoalCompletionReceipt(assistant, receipt); ok || attached != nil {
		t.Fatalf("mismatched receipt was attached: %+v", attached)
	}
}

func int64TestPointer(value int64) *int64 { return &value }

func optionalInt64Equal(left *int64, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func TestAttachResultSummaryRejectsDifferentAgentRound(t *testing.T) {
	assistant := protocol.Message{
		"message_id":     "assistant-1",
		"agent_id":       "agent-1",
		"round_id":       "round-root",
		"agent_round_id": "agent-round-1",
		"role":           "assistant",
		"content":        []map[string]any{{"type": "text", "text": "Agent1"}},
	}
	result := protocol.Message{
		"message_id":     "result-2",
		"agent_id":       "agent-2",
		"round_id":       "round-root",
		"agent_round_id": "agent-round-2",
		"role":           "result",
		"subtype":        "success",
		"result":         "Agent2",
	}

	if _, ok := AttachResultSummary(assistant, result); ok {
		t.Fatalf("不同 Agent 执行轮的 result 不应挂到 assistant")
	}
}

func TestBuildAssistantResultSummaryPreservesPermissionDenials(t *testing.T) {
	summary := BuildAssistantResultSummary(protocol.Message{
		"message_id": "result-1",
		"role":       "result",
		"subtype":    "success",
		"result":     "无法完成搜索：WebSearch 未被允许",
		"permission_denials": []map[string]any{{
			"tool_name": "WebSearch",
		}},
		"errors": []string{"permission denied"},
	}, "")

	denials, ok := summary["permission_denials"].([]map[string]any)
	if !ok || len(denials) != 1 || denials[0]["tool_name"] != "WebSearch" {
		t.Fatalf("permission_denials 未进入 result_summary: %+v", summary)
	}
	errorsValue, ok := summary["errors"].([]string)
	if !ok || len(errorsValue) != 1 || errorsValue[0] != "permission denied" {
		t.Fatalf("errors 未进入 result_summary: %+v", summary)
	}
}

func TestBuildAssistantResultSummaryPreservesRuntimeDiagnostics(t *testing.T) {
	summary := BuildAssistantResultSummary(protocol.Message{
		"message_id":        "result-diagnostics",
		"role":              "result",
		"subtype":           "error",
		"runtime_subtype":   "error_max_turns",
		"failure_phase":     "runtime_startup",
		"is_error":          true,
		"model_usage":       map[string]any{"glm-5.2": map[string]any{"input_tokens": 42}},
		"structured_output": map[string]any{"status": "failed"},
		"fast_mode_state":   "cooldown",
	}, "")

	if summary["runtime_subtype"] != "error_max_turns" || summary["fast_mode_state"] != "cooldown" {
		t.Fatalf("runtime diagnostics missing: %+v", summary)
	}
	if summary["failure_phase"] != "runtime_startup" {
		t.Fatalf("failure phase missing: %+v", summary)
	}
	if summary["model_usage"] == nil || summary["structured_output"] == nil {
		t.Fatalf("result payload diagnostics missing: %+v", summary)
	}
}

func TestProcessorPreservesResultPermissionDenials(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-denied",
		ParentID:   "round-denied",
	}, "sdk-session-denied")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-denied",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
			Result:  "无法完成搜索：WebSearch 未被允许",
			PermissionDenials: []sdkprotocol.PermissionDenial{{
				ToolName:  "WebSearch",
				ToolUseID: "tool-1",
				ToolInput: map[string]any{"query": "AI news"},
			}},
		},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("result durable 消息数量不正确: %+v", output.DurableMessages)
	}
	denials, ok := output.DurableMessages[0]["permission_denials"].([]map[string]any)
	if !ok || len(denials) != 1 {
		t.Fatalf("permission_denials 未保留: %+v", output.DurableMessages[0])
	}
	if denials[0]["tool_name"] != "WebSearch" || denials[0]["tool_use_id"] != "tool-1" {
		t.Fatalf("permission_denials 内容不正确: %+v", denials)
	}
	input, ok := denials[0]["tool_input"].(map[string]any)
	if !ok || input["query"] != "AI news" {
		t.Fatalf("permission_denials.tool_input 未保留: %+v", denials)
	}
}

func TestProcessorTreatsSuccessSubtypeWithErrorFlagAsFailure(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-error-flag",
		ParentID:   "round-error-flag",
	}, "sdk-session-error")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-error-flag",
		Result: &sdkprotocol.ResultMessage{
			Subtype:          "success",
			IsError:          true,
			Result:           "provider reported an error",
			ModelUsage:       map[string]any{"glm-5.2": map[string]any{"input_tokens": 42}},
			StructuredOutput: map[string]any{"status": "failed"},
			FastModeState:    "cooldown",
		},
	})

	if output.ResultSubtype != "error" || output.TerminalStatus != "error" {
		t.Fatalf("output terminal = (%q, %q), want error", output.ResultSubtype, output.TerminalStatus)
	}
	if len(output.DurableMessages) != 1 {
		t.Fatalf("durable messages = %+v", output.DurableMessages)
	}
	result := output.DurableMessages[0]
	if result["is_error"] != true || result["subtype"] != "error" || result["runtime_subtype"] != "success" {
		t.Fatalf("result error semantics = %+v", result)
	}
	if result["fast_mode_state"] != "cooldown" || result["structured_output"] == nil || result["model_usage"] == nil {
		t.Fatalf("result diagnostics missing = %+v", result)
	}
}

func TestProcessorNormalizesProviderContentFilterResultError(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:room:test",
		RoomID:     "room-test",
		AgentID:    "agent-test",
		RoundID:    "round-content-filtered",
	}, "sdk-session-content-filtered")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-content-filtered",
		Result: &sdkprotocol.ResultMessage{
			Subtype:        "error",
			IsError:        true,
			Result:         "request rejected",
			StopReason:     "sensitive",
			TerminalReason: "invalid_request",
			Errors:         []string{"generation stopped by Provider"},
		},
	})

	if output.TerminalStatus != "error" || output.ResultSubtype != "error" {
		t.Fatalf("content filter should remain a terminal error: %+v", output)
	}
	result := output.DurableMessages[0]
	if result["result"] != contentFilteredDisplayText {
		t.Fatalf("terminal result did not use fallback copy: %+v", result)
	}
	if !strings.Contains(contentFilteredDisplayText, "敏感信息") || !strings.Contains(contentFilteredDisplayText, "新建对话") {
		t.Fatalf("content filter copy must explain the sensitive-content risk and direct the user to a fresh conversation: %q", contentFilteredDisplayText)
	}
	if result["terminal_reason"] != contentFilteredTerminalReason {
		t.Fatalf("terminal reason was not normalized: %+v", result)
	}
	if result["stop_reason"] != "error" {
		t.Fatalf("Provider stop reason was not normalized: %+v", result)
	}
	errors, ok := result["errors"].([]string)
	if !ok || len(errors) != 1 || errors[0] != contentFilteredTerminalReason {
		t.Fatalf("raw terminal error should not remain user-visible: %+v", result)
	}
}

func TestProcessorWaitsForResultAfterAssistantAPIError(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-api-error",
	}, "sdk-session-api-error")

	assistantOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Error:      "authentication_failed",
			IsAPIError: true,
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-api-error",
				Model:      "<synthetic>",
				StopReason: "stop_sequence",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "Failed to authenticate. API Error: 401 invalid key"},
				},
			},
		},
	})
	if assistantOutput.TerminalStatus != "" || assistantOutput.ResultSubtype != "" || len(assistantOutput.DurableMessages) != 0 {
		t.Fatalf("API error assistant must not terminate the physical round: %+v", assistantOutput)
	}

	resultOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-api-error",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
			IsError: true,
			Result:  "Failed to authenticate. API Error: 401 invalid key",
		},
	})
	if resultOutput.TerminalStatus != "error" || resultOutput.ResultSubtype != "error" {
		t.Fatalf("final result should terminate as error: %+v", resultOutput)
	}
	if len(resultOutput.DurableMessages) != 1 {
		t.Fatalf("expected only the final durable result, got %+v", resultOutput.DurableMessages)
	}
	result := resultOutput.DurableMessages[0]
	if protocol.MessageRole(result) != "result" || result["is_error"] != true {
		t.Fatalf("final API error result semantics are invalid: %+v", result)
	}
}

func TestProcessorNormalizesHookStoppedResultError(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-hook-stopped"}, "")
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "error_hook_stopped",
			IsError:    true,
			StopReason: "hook_stopped",
			Errors:     []string{"Tool execution stopped by hook"},
		},
	})
	if output.ResultSubtype != "error" || output.TerminalStatus != "error" {
		t.Fatalf("hook stopped terminal = (%q, %q), want error", output.ResultSubtype, output.TerminalStatus)
	}
	result := output.DurableMessages[0]
	if result["runtime_subtype"] != "error_hook_stopped" {
		t.Fatalf("runtime subtype 未保留: %+v", result)
	}
	errors, ok := result["errors"].([]string)
	if !ok || len(errors) != 1 || errors[0] != hookStoppedDisplayText {
		t.Fatalf("hook stopped 错误未使用友好文案: %+v", result)
	}
	if result["result"] != hookStoppedDisplayText {
		t.Fatalf("hook stopped 应以 result 正文说明终止原因: %+v", result)
	}
	projected := ProjectResultMessage(nil, result)
	if projected == nil || ExtractAssistantDisplayText(projected) != hookStoppedDisplayText {
		t.Fatalf("hook stopped result 未投影为 assistant 正文: %+v", projected)
	}
}

// 只追加 transcript 的真实形态：assistant 逐块落盘且不带 stop_reason，round 终态只由 result 行承载。
func TestMergeHistoryResultSummariesSettlesAssistantFromResult(t *testing.T) {
	tests := []struct {
		name           string
		subtype        string
		wantStopReason string
	}{
		{name: "success", subtype: "success", wantStopReason: "end_turn"},
		{name: "error", subtype: "error", wantStopReason: "error"},
		{name: "interrupted", subtype: "interrupted", wantStopReason: "cancelled"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := []protocol.Message{
				{"message_id": "u1", "role": "user", "round_id": "round-1", "content": "hi"},
				{
					"message_id": "a1", "role": "assistant", "round_id": "round-1", "agent_round_id": "ar-1",
					"content": []any{map[string]any{"type": "thinking", "thinking": "plan"}}, "is_complete": false,
				},
				{
					"message_id": "a2", "role": "assistant", "round_id": "round-1", "agent_round_id": "ar-1",
					"content": []any{map[string]any{"type": "text", "text": "done"}}, "is_complete": false,
				},
				{
					// 同 root round 的另一 Agent 执行轮尚无 result，必须保持未完成。
					"message_id": "b1", "role": "assistant", "round_id": "round-1", "agent_round_id": "ar-2",
					"content": []any{map[string]any{"type": "text", "text": "still going"}}, "is_complete": false,
				},
				{
					"message_id": "r1", "role": "result", "round_id": "round-1", "agent_round_id": "ar-1",
					"subtype": test.subtype, "duration_ms": 1200, "result": "done",
				},
			}

			merged := MergeHistoryResultSummaries(rows)

			final := findHistoryRow(t, merged, "a2")
			if final["result_summary"] == nil {
				t.Fatalf("final assistant lost result_summary: %#v", final)
			}
			if final["is_complete"] != true {
				t.Fatalf("final assistant is_complete = %#v, want true", final["is_complete"])
			}
			if final["stop_reason"] != test.wantStopReason {
				t.Fatalf("final assistant stop_reason = %#v, want %q", final["stop_reason"], test.wantStopReason)
			}
			earlier := findHistoryRow(t, merged, "a1")
			if earlier["is_complete"] != true {
				t.Fatalf("earlier block of the settled agent round is_complete = %#v, want true", earlier["is_complete"])
			}
			if earlier["stop_reason"] != nil || earlier["result_summary"] != nil {
				t.Fatalf("terminal metadata must stay on the final block only: %#v", earlier)
			}
			if other := findHistoryRow(t, merged, "b1"); other["is_complete"] != false {
				t.Fatalf("another agent round without result must stay open, got %#v", other["is_complete"])
			}
			if rows[2]["is_complete"] != false {
				t.Fatalf("input history must stay untouched, got %#v", rows[2])
			}
		})
	}
}

func findHistoryRow(t *testing.T, rows []protocol.Message, messageID string) protocol.Message {
	t.Helper()
	for _, row := range rows {
		if row["message_id"] == messageID {
			return row
		}
	}
	t.Fatalf("message %q missing from merged history", messageID)
	return nil
}

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

func TestInterruptReasonClassification(t *testing.T) {
	for _, tc := range []struct {
		reason   string
		display  string
		explicit bool
	}{
		{InterruptWithoutMessage, "", true},
		{InterruptHostControlWithoutMessage, "", true},
		{"收到新的用户消息，上一轮已停止", "收到新的用户消息，上一轮已停止", false},
		{"host_shutdown", "host_shutdown", false},
		{"", "", false},
	} {
		if got := NormalizeInterruptDisplayText(tc.reason); got != tc.display {
			t.Fatalf("display(%q) = %q, want %q", tc.reason, got, tc.display)
		}
		if got := IsExplicitStopInterrupt(tc.reason); got != tc.explicit {
			t.Fatalf("explicit(%q) = %v, want %v", tc.reason, got, tc.explicit)
		}
	}
}

// TestPermissionReviewProjection 保证结构化审批事件生成独立审计消息。
func TestPermissionReviewProjection(t *testing.T) {
	p := &Processor{}
	message := sdkprotocol.SystemMessage{Subtype: "permission_review", Data: map[string]any{"tool_use_id": "tool-1", "review": map[string]any{"status": "approved", "risk": "low", "authorization": "high", "rationale": "已授权"}}}
	result := p.projectPermissionReviewSystemMessage(message)
	if result == nil || (*result)["content"] != "已自动批准：已授权" {
		t.Fatalf("message=%v", result)
	}
	if got := p.projectPermissionReviewSystemMessage(sdkprotocol.SystemMessage{Subtype: "permission_review"}); got != nil {
		t.Fatal("malformed review became a message")
	}
}

func TestIsRuntimeTurnStartOnlyMatchesTopLevelInit(t *testing.T) {
	parent := "toolu_parent"
	cases := []struct {
		name    string
		message sdkprotocol.ReceivedMessage
		want    bool
	}{
		{"top-level init", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, Subtype: "init"}, true},
		{"decoded init", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, System: &sdkprotocol.SystemMessage{Subtype: "init"}}, true},
		{"child init", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, Subtype: "init", ParentToolUseID: &parent}, false},
		{"task notification", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, Subtype: "task_notification"}, false},
		{"assistant", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}, false},
	}
	for _, tc := range cases {
		if got := IsRuntimeTurnStart(tc.message); got != tc.want {
			t.Fatalf("%s: IsRuntimeTurnStart = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// 内部 round 的用户行默认隐藏；装饰后必须作为可见触发行。
func TestDecorateRuntimeWakeUserMessageUnhidesTriggerRow(t *testing.T) {
	row := DecorateRuntimeWakeUserMessage(protocol.Message{"hidden_from_user": true, "content": "x"}, RuntimeWakeTrigger{TaskID: "b1", Status: "completed", Summary: "done"})
	if _, hidden := row["hidden_from_user"]; hidden || row["content"] != "done" || row["purpose"] != RuntimeWakePurpose {
		t.Fatalf("row = %+v", row)
	}
}

func roundTripJSON(t *testing.T, value any) any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return decodeRawJSON(encoded)
}

func TestCanonicalJSONValueMatchesRoundTrip(t *testing.T) {
	cases := []any{
		nil, true, "text <&>  ", float64(1.5), int(7), int64(1<<53 + 1), int32(-3),
		map[string]any(nil), []any(nil), []map[string]any(nil), []string(nil),
		map[string]any{"a": []any{1, "x", nil, map[string]any{"b": false}}, "c": []map[string]any{{"d": 2.25}, nil}},
		[]string{"a", "b"},
	}
	random := rand.New(rand.NewPCG(3, 4))
	var build func(depth int) any
	build = func(depth int) any {
		switch kind := random.IntN(8); {
		case depth > 3 || kind < 3:
			return []any{"s", float64(random.IntN(100)) / 8, int64(random.Int64()), true, nil}[random.IntN(5)]
		case kind < 5:
			value := map[string]any{}
			for i := range random.IntN(4) {
				value[string(rune('a'+i))] = build(depth + 1)
			}
			return value
		case kind < 7:
			value := []any{}
			for range random.IntN(4) {
				value = append(value, build(depth+1))
			}
			return value
		default:
			return []map[string]any{{"k": build(depth + 1)}}
		}
	}
	for range 300 {
		cases = append(cases, build(0))
	}
	for _, value := range cases {
		got, ok := canonicalJSONValue(value)
		if !ok {
			t.Fatalf("%#v unexpectedly rejected", value)
		}
		if want := roundTripJSON(t, value); !reflect.DeepEqual(got, want) {
			t.Fatalf("%#v: got %#v want %#v", value, got, want)
		}
	}
	for _, value := range []any{"bad\xff", map[string]any{"\xff": 1}, math.NaN(), math.Inf(1), float32(0.1), []string{"\xc3"}, struct{}{}} {
		if _, ok := canonicalJSONValue(value); ok {
			t.Fatalf("%#v must fall back to the encoding round trip", value)
		}
	}
}

func TestDecodeBlockJSONFieldMatchesEncodedContent(t *testing.T) {
	content := []any{map[string]any{"type": "text", "text": "out\xff"}, map[string]any{"type": "text", "text": "ok"}}
	message, err := sdkprotocol.DecodeMessage(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
		map[string]any{"type": "tool_result", "tool_use_id": "t", "content": content},
		map[string]any{"type": "tool_use", "id": "u", "name": "Bash", "input": map[string]any{"command": "ls", "n": 3}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range message.User.Message.Content {
		if result, ok := sdkprotocol.AsToolResultBlock(block); ok {
			if got, want := toolResultContentValue(result), decodeRawJSON(result.ContentJSON()); !reflect.DeepEqual(got, want) {
				t.Fatalf("tool_result content: got %#v want %#v", got, want)
			}
		}
		if use, ok := sdkprotocol.AsToolUseBlock(block); ok {
			if got, want := decodeBlockJSONField(block, "input", use.Input), decodeRawJSON(use.Input); !reflect.DeepEqual(got, want) {
				t.Fatalf("tool_use input: got %#v want %#v", got, want)
			}
		}
	}
	// SDK 外部构造、没有原始 payload 的 block 仍按编码字段解码。
	block := sdkprotocol.ToolResultBlock{Content: json.RawMessage(`{"a":1}`)}
	if got := toolResultContentValue(block); !reflect.DeepEqual(got, map[string]any{"a": float64(1)}) {
		t.Fatalf("encoded fallback: %#v", got)
	}
}

func TestConfigurationToolSecretsAreRedactedFromStreamStartAndDelta(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:stream-secret-redaction",
		AgentID:    "nexus",
		RoundID:    "round-stream-secret-redaction",
	}, "sdk-stream-secret-redaction")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-stream-secret-redaction"},
		}},
	})

	start := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type": "tool_use",
				"id":   "tool-stream-secret-redaction",
				"name": "mcp__nexus_config__apply_nexus_configuration_change",
				"input": map[string]any{
					"domain": "providers",
					"input": map[string]any{
						"auth_token": "topsecret-start-987",
					},
				},
			},
		}},
	})
	if len(start.StreamEvents) != 1 {
		t.Fatalf("start stream events = %#v", start.StreamEvents)
	}
	assertConfigurationProjectionRedacted(t, start.StreamEvents[0].Data)

	deltaProcessor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:stream-delta-secret-redaction",
		AgentID:    "nexus",
		RoundID:    "round-stream-delta-secret-redaction",
	}, "sdk-stream-delta-secret-redaction")
	deltaProcessor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-stream-delta-secret-redaction"},
		}},
	})
	deltaProcessor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    "tool-stream-delta-secret-redaction",
				"name":  "mcp__nexus_config__apply_nexus_configuration_change",
				"input": map[string]any{},
			},
		}},
	})
	delta := deltaProcessor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type": "input_json_delta",
				"partial_json": `{
					"domain":"providers",
					"input":{"auth_token":"topsecret-delta-987"}
				}`,
			},
		}},
	})
	if len(delta.StreamEvents) != 1 {
		t.Fatalf("delta stream events = %#v", delta.StreamEvents)
	}
	assertConfigurationProjectionRedacted(t, delta.StreamEvents[0].Data)
}

func assertConfigurationProjectionRedacted(t *testing.T, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	for _, forbidden := range []string{
		"topsecret-final-987",
		"topsecret-start-987",
		"topsecret-delta-987",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("configuration projection leaked %q: %s", forbidden, payload)
		}
	}
}
