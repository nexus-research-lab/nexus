package dm

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (r *roundRunner) dispatchGoalContinuation(ctx context.Context) {
	if r.service.goals == nil || len(r.service.runtime.GetRunningRoundIDs(r.sessionKey)) > 0 {
		return
	}
	plan, err := r.service.goals.PlanContinuationForSession(ctx, r.sessionKey, r.roundID)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) || errors.Is(err, goalsvc.ErrGoalVersionStale) {
			return
		}
		r.service.loggerFor(ctx).Warn("规划 Goal 自动续跑失败",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
			"err", err,
		)
		return
	}
	if plan == nil {
		return
	}
	if err := r.service.HandleChat(ctx, Request{
		SessionKey:           r.sessionKey,
		AgentID:              r.agent.AgentID,
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
	}); err != nil {
		r.service.loggerFor(ctx).Warn("启动 Goal 自动续跑失败",
			"session_key", r.sessionKey,
			"round_id", plan.RoundID,
			"goal_id", plan.Goal.ID,
			"err", err,
		)
	}
}
