package sessionresume

import (
	"errors"
	"testing"
)

func TestPolicyCanPersistAllowsNonTranscriptSessionID(t *testing.T) {
	policy := NewPolicy(fakeTranscriptStore{})

	decision := policy.CanPersist("/workspace", "sdk-session-1")

	if !decision.Allowed {
		t.Fatalf("非 transcript 形态 session id 应允许持久化测试/legacy 值: %+v", decision)
	}
	if decision.Reason != ReasonNonTranscriptSession {
		t.Fatalf("原因不正确: %+v", decision)
	}
}

func TestPolicyCanResumeRequiresTranscript(t *testing.T) {
	sessionID := "11111111-1111-4111-8111-111111111111"
	policy := NewPolicy(fakeTranscriptStore{exists: map[string]bool{sessionID: true}})

	allowed := policy.CanResume("/workspace", sessionID)
	if !allowed.Allowed || allowed.Reason != ReasonTranscriptExists {
		t.Fatalf("存在 transcript 时应允许 resume: %+v", allowed)
	}

	missing := policy.CanResume("/workspace", "22222222-2222-4222-8222-222222222222")
	if missing.Allowed || missing.Reason != ReasonTranscriptMissing {
		t.Fatalf("缺失 transcript 时不应允许 resume: %+v", missing)
	}
}

func TestPolicyCanPersistBlocksTranscriptCheckError(t *testing.T) {
	checkErr := errors.New("stat failed")
	policy := NewPolicy(fakeTranscriptStore{err: checkErr})

	decision := policy.CanPersist("/workspace", "33333333-3333-4333-8333-333333333333")

	if decision.Allowed {
		t.Fatalf("transcript 检查失败时不应写入 resume: %+v", decision)
	}
	if !errors.Is(decision.Err, checkErr) || decision.Reason != ReasonTranscriptCheckFailed {
		t.Fatalf("检查失败原因不正确: %+v", decision)
	}
}

type fakeTranscriptStore struct {
	exists map[string]bool
	err    error
}

func (s fakeTranscriptStore) TranscriptSessionExists(_ string, sessionID string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.exists[sessionID], nil
}
