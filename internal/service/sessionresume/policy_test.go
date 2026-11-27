package sessionresume

import (
	"errors"
	"testing"
)

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
