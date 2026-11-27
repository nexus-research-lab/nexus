package runtimeadmission

import (
	"context"
	"errors"
	"testing"
)

func TestTransitionFailureDoesNotCommitAndReopensAdmission(t *testing.T) {
	gate := NewGate()
	revokeErr := errors.New("runtime revoke failed")
	committed := false
	err := gate.Transition(
		context.Background(),
		func(context.Context) error { return revokeErr },
		func(context.Context) error {
			committed = true
			return nil
		},
	)
	if !errors.Is(err, revokeErr) {
		t.Fatalf("Transition() error = %v, want revoke error", err)
	}
	if committed {
		t.Fatal("revoke 失败后不应提交安全边界")
	}
	lease, err := gate.Admit(context.Background())
	if err != nil {
		t.Fatalf("失败转场后 admission 未重新开放: %v", err)
	}
	lease.Release()
}

// NewGate 创建 runtime admission gate。
func NewGate() *Gate {
	return &Gate{active: make(map[uint64]context.CancelFunc)}
}
