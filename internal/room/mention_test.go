package room

import (
	"testing"
)

func TestResolveMentionAgentIDsKeepsAllMentionedMembers(t *testing.T) {
	aliases := map[string]string{
		"Amy":   "agent-amy",
		"Devin": "agent-devin",
	}

	for _, content := range []string{
		"@Amy @Devin 分别给一个建议",
		"@Amy 输出完后 @Devin 再回答",
		"@Amy 让 @Devin 查天气并在公区回复",
	} {
		got := ResolveMentionAgentIDs(content, aliases)
		want := []string{"agent-amy", "agent-devin"}
		if !sameStringSet(got, want) {
			t.Fatalf("Room @ 目标解析不应按自然语言裁剪: content=%q got=%v want=%v", content, got, want)
		}
	}
}

func TestResolveMentionAgentIDsPreservesTextOrder(t *testing.T) {
	aliases := map[string]string{
		"Amy":   "agent-amy",
		"Devin": "agent-devin",
		"sam":   "agent-sam",
	}

	got := ResolveMentionAgentIDs("@sam 先来，然后 @Amy 收一下，最后 @Devin 总结", aliases)
	want := []string{"agent-sam", "agent-amy", "agent-devin"}
	if !sameStringSlice(got, want) {
		t.Fatalf("Room @ 目标解析应按文本顺序返回: got=%v want=%v", got, want)
	}
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]int, len(left))
	for _, item := range left {
		seen[item]++
	}
	for _, item := range right {
		seen[item]--
		if seen[item] < 0 {
			return false
		}
	}
	return true
}

func sameStringSlice(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
