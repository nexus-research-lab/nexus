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
