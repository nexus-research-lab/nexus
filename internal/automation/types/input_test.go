package types

import "testing"

func TestCreateJobInputNormalizesOverlapPolicy(t *testing.T) {
	input := CreateJobInput{
		Name:        "任务",
		AgentID:     "agent-1",
		Instruction: "执行",
		Schedule: Schedule{
			Kind:            ScheduleKindEvery,
			IntervalSeconds: func() *int { value := 60; return &value }(),
		},
		SessionTarget: SessionTarget{Kind: SessionTargetIsolated},
		Delivery:      DeliveryTarget{Mode: DeliveryModeNone},
	}
	if got := input.Normalized().OverlapPolicy; got != OverlapPolicySkip {
		t.Fatalf("默认 overlap_policy 应为 skip，实际 %s", got)
	}
	invalid := input
	invalid.OverlapPolicy = "queue"
	if err := invalid.Normalized().Validate(); err == nil {
		t.Fatalf("非法 overlap_policy 应被拒绝")
	}
}
