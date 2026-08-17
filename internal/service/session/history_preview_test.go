package session

import (
	"context"
	"testing"

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
