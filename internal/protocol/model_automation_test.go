package protocol

import "testing"

func TestSessionTargetValidateRejectsNamedMain(t *testing.T) {
	target := SessionTarget{
		Kind:            SessionTargetNamed,
		NamedSessionKey: "main",
	}.Normalized()
	if err := target.Validate(); err == nil {
		t.Fatalf("named_session_key=main 应被拒绝")
	}
}

func TestSourceValidateRequiresContextTypeAndContextIDPair(t *testing.T) {
	sourceWithContextIDOnly := Source{
		Kind:      SourceKindUserPage,
		ContextID: "chat-1",
	}.Normalized()
	if err := sourceWithContextIDOnly.Validate(); err == nil {
		t.Fatalf("context_id 存在时应要求 context_type")
	}

	sourceWithContextTypeOnly := Source{
		Kind:        SourceKindUserPage,
		ContextType: "chat",
	}.Normalized()
	if err := sourceWithContextTypeOnly.Validate(); err == nil {
		t.Fatalf("context_type 存在时应要求 context_id")
	}

	validSource := Source{
		Kind:         SourceKindUserPage,
		ContextType:  "chat",
		ContextID:    "chat-1",
		ContextLabel: "Room 1",
	}.Normalized()
	if err := validSource.Validate(); err != nil {
		t.Fatalf("合法 source 校验失败: %v", err)
	}

	roomSource := Source{
		Kind:         SourceKindUserPage,
		ContextType:  "room",
		ContextID:    "room-1",
		ContextLabel: "Room 1",
	}.Normalized()
	if err := roomSource.Validate(); err != nil {
		t.Fatalf("room source 校验失败: %v", err)
	}
}

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
