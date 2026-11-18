// INPUT: 含转义字符、HTML 字符、行分隔符、非法 UTF-8 与多字节字符的字符串。
// OUTPUT: 直接计数的 JSON 编码长度与 encoding/json 完全一致。
// POS: 大型工具输出判定快路径的回归。
package workspace

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
)

func TestHistoryJSONStringEncodedSizeMatchesEncodingJSON(t *testing.T) {
	cases := []string{
		"", "plain", "quote\" back\\slash", "\b\f\n\r\t", "\x00\x01\x1f\x7f",
		"<script>&amp;</script>", "line para ", "bad\xffutf8\xc3", "中文😀é",
		strings.Repeat("x", 9000),
	}
	alphabet := []string{"a", "\"", "\\", "\n", "\x02", "<", "&", " ", "\xff", "\xe4\xb8", "中", "😀", "\x7f"}
	random := rand.New(rand.NewPCG(1, 2))
	for range 500 {
		var builder strings.Builder
		for range random.IntN(40) {
			builder.WriteString(alphabet[random.IntN(len(alphabet))])
		}
		cases = append(cases, builder.String())
	}
	for _, text := range cases {
		payload, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		if got := historyJSONStringEncodedSize(text); got != len(payload) {
			t.Fatalf("%q: got %d want %d", text, got, len(payload))
		}
		if got, err := historyJSONEncodedSize(any(text)); err != nil || got != len(payload) {
			t.Fatalf("%q: generic size %d err %v", text, got, err)
		}
	}
}
