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
}
