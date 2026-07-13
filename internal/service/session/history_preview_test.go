package session

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestLatestReplyPreviewUsesNewestVisibleAssistantText(t *testing.T) {
	messages := []protocol.Message{
		{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "较早的回复"},
			},
		},
		{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "被中断的回复"},
			},
			"result_summary": map[string]any{"subtype": "interrupted"},
		},
		{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "thinking", "thinking": "不应进入摘要"},
				{"type": "text", "text": "  最新回复\n包含多行内容  "},
			},
		},
	}

	if got := latestReplyPreview(messages); got != "最新回复 包含多行内容" {
		t.Fatalf("latestReplyPreview() = %q", got)
	}
}

func TestLatestReplyPreviewUsesResultTextAndLimitsRunes(t *testing.T) {
	resultText := strings.Repeat("界", latestReplyPreviewRuneLimit+20)
	messages := []protocol.Message{
		{
			"role":           "assistant",
			"result_summary": map[string]any{"subtype": "success", "result": resultText},
		},
	}

	got := latestReplyPreview(messages)
	if utf8.RuneCountInString(got) != latestReplyPreviewRuneLimit {
		t.Fatalf("摘要长度 = %d, want %d", utf8.RuneCountInString(got), latestReplyPreviewRuneLimit)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("超长摘要缺少省略号: %q", got)
	}
}

func TestLatestReplyPreviewSupportsStringContent(t *testing.T) {
	messages := []protocol.Message{
		{"role": "assistant", "content": "字符串形式的回复"},
	}

	if got := latestReplyPreview(messages); got != "字符串形式的回复" {
		t.Fatalf("latestReplyPreview() = %q", got)
	}
}
