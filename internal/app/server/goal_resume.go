package server

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type goalContinuationDispatcher struct {
	runtime *runtimectx.Manager
	dm      *dmsvc.Service
}

func newGoalContinuationDispatcher(runtime *runtimectx.Manager, dm *dmsvc.Service) *goalContinuationDispatcher {
	return &goalContinuationDispatcher{runtime: runtime, dm: dm}
}

func (d *goalContinuationDispatcher) ShouldDeferGoalContinuation(ctx context.Context, sessionKey string) bool {
	sessionKey = strings.TrimSpace(sessionKey)
	if d == nil || sessionKey == "" {
		return true
	}
	if d.runtime != nil && len(d.runtime.GetRunningRoundIDs(sessionKey)) > 0 {
		return true
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindAgent || strings.TrimSpace(parsed.AgentID) == "" {
		return true
	}
	if d.dm == nil {
		return true
	}
	return d.dm.ShouldDeferGoalContinuation(ctx, sessionKey, parsed.AgentID)
}

func (d *goalContinuationDispatcher) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	if d == nil || d.dm == nil {
		return errors.New("goal continuation dispatcher is not configured")
	}
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindAgent || strings.TrimSpace(parsed.AgentID) == "" {
		return errors.New("goal continuation only supports agent session keys")
	}
	return d.dm.HandleChat(ctx, dmsvc.Request{
		SessionKey:           sessionKey,
		AgentID:              parsed.AgentID,
		Content:              plan.Prompt,
		RoundID:              plan.RoundID,
		ReqID:                plan.RoundID,
		DeliveryPolicy:       protocol.ChatDeliveryPolicyQueue,
		BroadcastUserMessage: false,
		Internal:             true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Synthetic:      plan.Synthetic,
			HiddenFromUser: plan.HiddenFromUser,
			Purpose:        plan.Purpose,
			Priority:       "internal",
			Metadata:       plan.Metadata,
		},
	})
}
