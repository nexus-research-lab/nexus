package clientopts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

type fakeRuntimeAdmissionResolver struct {
	required bool
	err      error
}

func (f fakeRuntimeAdmissionResolver) BeginAgentRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, bool, error) {
	return runtimeadmission.NewDetachedLease(ctx), f.required, f.err
}

func TestBeginAgentRuntimeAdmissionFailsClosed(t *testing.T) {
	lease, required, err := BeginAgentRuntimeAdmission(
		context.Background(),
		fakeRuntimeAdmissionResolver{err: errors.New("database unavailable")},
	)
	if lease != nil || !required || err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("认证状态读取失败必须 fail closed，lease=%v required=%v err=%v", lease, required, err)
	}
}

func TestBuildAgentClientOptionsRejectsRequiredIsolationOff(t *testing.T) {
	_, err := BuildAgentClientOptions(
		context.Background(),
		fakeRuntimeConfigResolver{},
		AgentClientOptionsInput{
			RuntimeIsolationMode:     "off",
			RuntimeIsolationRequired: true,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "requires runtime isolation enforce") {
		t.Fatalf("认证部署未启用 enforce 应拒绝 Agent runtime，err=%v", err)
	}
}
