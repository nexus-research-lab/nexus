package session

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type goalCompletionUsageProviderStub struct {
	report *protocol.GoalUsageReport
	calls  int
}

func (p *goalCompletionUsageProviderStub) UsageByGoalIDForOwner(
	_ context.Context,
	_ string,
	_ string,
) (*protocol.GoalUsageReport, error) {
	p.calls++
	return p.report, nil
}

func TestRefreshGoalCompletionReceiptsUsesCurrentAggregateTruth(t *testing.T) {
	provider := &goalCompletionUsageProviderStub{report: &protocol.GoalUsageReport{
		GoalID:          "goal-receipt",
		Status:          protocol.GoalStatusComplete,
		UsageFinalized:  true,
		TimeUsedSeconds: 1025,
		Usage: protocol.GoalUsage{
			InputTokens:          111082,
			OutputTokens:         7080,
			CacheReadInputTokens: 435968,
			ActualTotalTokens:    554130,
			ActualTotalKnown:     true,
		},
	}}
	service := &Service{goalUsage: provider}
	items := []protocol.Message{
		{
			"message_id": "assistant-final",
			protocol.GoalCompletionReceiptField: map[string]any{
				"goal_id":           "goal-receipt",
				"round_id":          "round-final",
				"time_used_seconds": 1025,
				"actual_tokens":     0,
			},
		},
		{
			"message_id": "assistant-same-goal",
			protocol.GoalCompletionReceiptField: protocol.GoalCompletionReceipt{
				GoalID: "goal-receipt", RoundID: "round-peer",
			},
		},
	}

	refreshed := service.refreshGoalCompletionReceipts(context.Background(), items)
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want one per Goal", provider.calls)
	}
	for index, item := range refreshed {
		receipt, ok := protocol.GoalCompletionReceiptFromAny(
			item[protocol.GoalCompletionReceiptField],
		)
		if !ok || receipt.ActualTokens == nil || *receipt.ActualTokens != 554130 {
			t.Fatalf("receipt[%d] = %#v, want refreshed actual 554130", index, item)
		}
	}
	restored, ok := protocol.GoalCompletionReceiptFromAny(
		items[0][protocol.GoalCompletionReceiptField],
	)
	if !ok || restored.ActualTokens == nil || *restored.ActualTokens != 0 {
		t.Fatalf("input receipt mutated = %#v, want original zero intact", items[0])
	}
}

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
