package clientopts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

type fakeRuntimeAdmissionResolver struct {
	err error
}

func (f fakeRuntimeAdmissionResolver) BeginAgentRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, error) {
	return runtimeadmission.NewDetachedLease(ctx), f.err
}

func TestBeginAgentRuntimeAdmissionFailsClosed(t *testing.T) {
	lease, err := BeginAgentRuntimeAdmission(
		context.Background(),
		fakeRuntimeAdmissionResolver{err: errors.New("database unavailable")},
	)
	if lease != nil || err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("认证状态读取失败必须 fail closed，lease=%v err=%v", lease, err)
	}
}

func TestBuildAgentClientOptionsHonorsConfiguredOffIsolationMode(t *testing.T) {
	ctx := authctx.WithState(context.Background(), authctx.State{AuthRequired: true})
	ctx = authctx.WithPrincipal(ctx, &authctx.Principal{UserID: "owner-a"})
	_, err := BuildAgentClientOptions(
		ctx,
		fakeRuntimeConfigResolver{},
		AgentClientOptionsInput{
			OwnerUserID:          "owner-a",
			WorkspacePath:        "/tmp/workspace",
			RuntimeIsolationMode: "off",
		},
	)
	if err != nil {
		t.Fatalf("off 配置应直接生效，err=%v", err)
	}
}
