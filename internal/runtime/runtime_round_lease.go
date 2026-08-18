// INPUT: DM round 或 Room Agent slot 的真实 runtime session/round lease。
// OUTPUT: 只在服务内部传播、不可由模型输入覆盖的 lease context。
// POS: runtime command 与进程内 capability builder 的共享鉴权上下文。
package runtime

import (
	"context"
	"strings"
)

type runtimeRoundLeaseContextKey struct{}

// RuntimeRoundLease 是 runtime Manager 实际登记的 session/round 对。
type RuntimeRoundLease struct {
	SessionKey string
	RoundID    string
}

// WithRuntimeRoundLease 把真实 runtime lease 传给进程内 capability builder。
func WithRuntimeRoundLease(ctx context.Context, sessionKey string, roundID string) context.Context {
	lease := RuntimeRoundLease{
		SessionKey: strings.TrimSpace(sessionKey),
		RoundID:    strings.TrimSpace(roundID),
	}
	if lease.SessionKey == "" || lease.RoundID == "" {
		return ctx
	}
	return context.WithValue(ctx, runtimeRoundLeaseContextKey{}, lease)
}

// RuntimeRoundLeaseFromContext 读取服务内部注入的真实 runtime lease。
func RuntimeRoundLeaseFromContext(ctx context.Context) (RuntimeRoundLease, bool) {
	lease, ok := ctx.Value(runtimeRoundLeaseContextKey{}).(RuntimeRoundLease)
	if !ok || strings.TrimSpace(lease.SessionKey) == "" || strings.TrimSpace(lease.RoundID) == "" {
		return RuntimeRoundLease{}, false
	}
	return lease, true
}
