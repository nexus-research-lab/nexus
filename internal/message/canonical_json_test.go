// INPUT: 嵌套 JSON 形状值、整数、非法 UTF-8、非有限数与 typed slice。
// OUTPUT: canonicalJSONValue 与 Marshal→Unmarshal 往返结果一致，无法一致时显式回退。
// POS: 工具输出免往返复制的等价性回归。
package message

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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
			if got, want := decodeBlockJSONField(block, "content", result.Content), decodeRawJSON(result.Content); !reflect.DeepEqual(got, want) {
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
	if got := decodeBlockJSONField(block, "content", block.Content); !reflect.DeepEqual(got, map[string]any{"a": float64(1)}) {
		t.Fatalf("encoded fallback: %#v", got)
	}
}
