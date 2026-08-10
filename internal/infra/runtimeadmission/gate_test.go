package runtimeadmission

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestTransitionCancelsAndDrainsAdmissionsBeforeRevokeAndCommit(t *testing.T) {
	gate := NewGate()
	active, err := gate.Admit(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var (
		orderMu sync.Mutex
		order   []string
	)
	revokeStarted := make(chan struct{})
	revokeContinue := make(chan struct{})
	transitionDone := make(chan error, 1)
	go func() {
		transitionDone <- gate.Transition(
			context.Background(),
			func(context.Context) error {
				orderMu.Lock()
				order = append(order, "revoke")
				orderMu.Unlock()
				close(revokeStarted)
				<-revokeContinue
				return nil
			},
			func(context.Context) error {
				orderMu.Lock()
				order = append(order, "commit")
				orderMu.Unlock()
				return nil
			},
		)
	}()

	select {
	case <-active.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("安全转场未取消在途 admission")
	}
	select {
	case <-revokeStarted:
		t.Fatal("在途 admission 未释放前不应开始 revoke")
	default:
	}

	waitingAdmission := make(chan *Lease, 1)
	go func() {
		lease, admitErr := gate.Admit(context.Background())
		if admitErr == nil {
			waitingAdmission <- lease
		}
	}()
	select {
	case <-waitingAdmission:
		t.Fatal("安全转场期间不应放行新 admission")
	default:
	}

	active.Release()
	select {
	case <-revokeStarted:
	case <-time.After(time.Second):
		t.Fatal("排空 admission 后未开始 revoke")
	}
	select {
	case <-waitingAdmission:
		t.Fatal("revoke 未完成前不应放行新 admission")
	default:
	}

	close(revokeContinue)
	select {
	case err = <-transitionDone:
		if err != nil {
			t.Fatalf("Transition() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("安全转场未完成")
	}
	select {
	case lease := <-waitingAdmission:
		lease.Release()
	case <-time.After(time.Second):
		t.Fatal("安全转场完成后未重新开放 admission")
	}

	orderMu.Lock()
	defer orderMu.Unlock()
	if len(order) != 2 || order[0] != "revoke" || order[1] != "commit" {
		t.Fatalf("转场顺序 = %v, want [revoke commit]", order)
	}
}

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

func TestLeaseReleaseCancelsContext(t *testing.T) {
	for _, test := range []struct {
		name  string
		lease func() *Lease
	}{
		{
			name: "gated",
			lease: func() *Lease {
				lease, err := NewGate().Admit(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				return lease
			},
		},
		{
			name:  "detached",
			lease: func() *Lease { return NewDetachedLease(context.Background()) },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			lease := test.lease()
			lease.Release()
			select {
			case <-lease.Context().Done():
			case <-time.After(time.Second):
				t.Fatal("Release() 未取消 lease context")
			}
		})
	}
}
