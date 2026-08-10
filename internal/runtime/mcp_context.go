// INPUT: DM round 或 Room Agent slot 的真实 runtime session/round lease。
// OUTPUT: 只在服务内部传播、不可由 MCP 参数覆盖的 lease context。
// POS: MCP builder 的共享 runtime 鉴权上下文；业务 source context 与 lease 身份保持分离。
package runtime

import (
	"context"
	"strings"
)

type mcpRoundLeaseContextKey struct{}

// MCPRoundLease 是 runtime Manager 实际登记的 session/round 对。
type MCPRoundLease struct {
	SessionKey string
	RoundID    string
}

// WithMCPRoundLease 把真实 runtime lease 传给 in-process MCP builder。
func WithMCPRoundLease(ctx context.Context, sessionKey string, roundID string) context.Context {
	lease := MCPRoundLease{
		SessionKey: strings.TrimSpace(sessionKey),
		RoundID:    strings.TrimSpace(roundID),
	}
	if lease.SessionKey == "" || lease.RoundID == "" {
		return ctx
	}
	return context.WithValue(ctx, mcpRoundLeaseContextKey{}, lease)
}

// MCPRoundLeaseFromContext 读取服务内部注入的真实 runtime lease。
func MCPRoundLeaseFromContext(ctx context.Context) (MCPRoundLease, bool) {
	lease, ok := ctx.Value(mcpRoundLeaseContextKey{}).(MCPRoundLease)
	if !ok || strings.TrimSpace(lease.SessionKey) == "" || strings.TrimSpace(lease.RoundID) == "" {
		return MCPRoundLease{}, false
	}
	return lease, true
}
