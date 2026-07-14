// INPUT: Goal continuation plan 与 DM/Room dispatcher。
// OUTPUT: 目标存在性、延迟判断和经最终校验的运行时派发。
// POS: app server 的 Goal 恢复装配层，不承载会话启动规则。
package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type goalContinuationDM interface {
	ShouldDeferGoalContinuation(context.Context, string, string) bool
	GoalContinuationTargetMissing(context.Context, string, string) (bool, error)
	DispatchGoalContinuation(context.Context, protocol.GoalContinuation) error
}

type goalContinuationRoom interface {
	ShouldDeferGoalContinuation(context.Context, string) bool
	GoalContinuationTargetMissing(context.Context, string) (bool, error)
	GoalContinuationConversationMissing(context.Context, string) (bool, error)
	DispatchGoalContinuation(context.Context, protocol.GoalContinuation) error
}

type goalContinuationDispatcher struct {
	runtime *runtimectx.Manager
	dm      goalContinuationDM
	room    goalContinuationRoom
}

func newGoalContinuationDispatcher(runtime *runtimectx.Manager, dm *dmsvc.Service, room *roomsvc.RealtimeService) *goalContinuationDispatcher {
	return &goalContinuationDispatcher{runtime: runtime, dm: dm, room: room}
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
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) == "" || d.dm == nil {
			return true
		}
		return d.dm.ShouldDeferGoalContinuation(ctx, sessionKey, parsed.AgentID)
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return true
		}
		return d.room.ShouldDeferGoalContinuation(ctx, sessionKey)
	default:
		return true
	}
}

func (d *goalContinuationDispatcher) GoalContinuationTargetMissing(ctx context.Context, sessionKey string) (bool, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if d == nil || sessionKey == "" {
		return false, nil
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured {
		return true, nil
	}
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) == "" {
			return true, nil
		}
		if parsed.ChatType == "group" && strings.TrimSpace(parsed.Ref) != "" && d.room != nil {
			missing, err := d.room.GoalContinuationConversationMissing(ctx, parsed.Ref)
			if err != nil || missing {
				return missing, err
			}
		}
		if d.dm == nil {
			return false, nil
		}
		return d.dm.GoalContinuationTargetMissing(ctx, sessionKey, parsed.AgentID)
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return false, nil
		}
		return d.room.GoalContinuationTargetMissing(ctx, sessionKey)
	default:
		return true, nil
	}
}

func (d *goalContinuationDispatcher) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	if d == nil {
		return errors.New("goal continuation dispatcher is not configured")
	}
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) == "" || d.dm == nil {
			return errors.New("goal continuation requires an agent session dispatcher")
		}
		err := d.dm.DispatchGoalContinuation(ctx, plan)
		if errors.Is(err, agentsvc.ErrAgentNotFound) {
			return fmt.Errorf("%w: %v", goalsvc.ErrGoalContinuationTargetMissing, err)
		}
		return err
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return errors.New("goal continuation requires a room session dispatcher")
		}
		err := d.room.DispatchGoalContinuation(ctx, plan)
		if errors.Is(err, roomsvc.ErrRoomNotFound) || errors.Is(err, roomsvc.ErrConversationNotFound) {
			return fmt.Errorf("%w: %v", goalsvc.ErrGoalContinuationTargetMissing, err)
		}
		return err
	default:
		return errors.New("goal continuation only supports agent or room session keys")
	}
}
