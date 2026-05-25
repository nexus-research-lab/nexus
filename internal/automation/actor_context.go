package automation

import (
	"context"
	"strings"
)

type actorAgentContextKey struct{}

// WithActorAgentID 标记本次自动化管理动作由哪个 Agent 发起。
func WithActorAgentID(ctx context.Context, agentID string) context.Context {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return ctx
	}
	return context.WithValue(ctx, actorAgentContextKey{}, agentID)
}

// ActorAgentID 从上下文读取本次自动化管理动作的发起 Agent。
func ActorAgentID(ctx context.Context) (string, bool) {
	agentID, _ := ctx.Value(actorAgentContextKey{}).(string)
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "", false
	}
	return agentID, true
}
