// INPUT: Goal steering、暂停与 durable continuation 计划。
// OUTPUT: DM/Room runtime guidance、精确中断与最终续跑派发。
// POS: Goal service 与 DM/Room runtime 生命周期之间的应用层适配器。
package goal

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
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
)

type goalGuidanceDispatcher struct {
	runtime *runtimectx.Manager
	room    *roomrealtime.Service
}

// NewGuidanceDispatcher 创建 Goal 到 DM/Room runtime 的 guidance 适配器。
func NewGuidanceDispatcher(
	runtime *runtimectx.Manager,
	room *roomrealtime.Service,
) *goalGuidanceDispatcher {
	return &goalGuidanceDispatcher{runtime: runtime, room: room}
}

func (d goalGuidanceDispatcher) QueueGuidanceInput(ctx context.Context, sessionKey string, roundID string, content string) ([]string, error) {
	return d.runtime.QueueGuidanceInput(ctx, sessionKey, roundID, content)
}

func (d goalGuidanceDispatcher) QueueContextualGuidanceInput(ctx context.Context, sessionKey string, roundID string, contextName string, content string, objectiveRevision int64) ([]string, error) {
	if protocol.IsRoomSharedSessionKey(sessionKey) && d.room != nil {
		return d.room.QueueRoomContextualGuidanceInput(ctx, sessionKey, roundID, contextName, content, "", objectiveRevision)
	}
	return d.runtime.QueueGoalContextualGuidanceInputOnConsumed(ctx, sessionKey, roundID, contextName, content, func() {
		d.runtime.AdoptGoalObjectiveRevision(sessionKey, objectiveRevision)
	})
}

func (d goalGuidanceDispatcher) QueueRoomContextualGuidanceInput(ctx context.Context, sessionKey string, roundID string, contextName string, content string, excludedAgentID string, objectiveRevision int64) ([]string, error) {
	if d.room != nil {
		return d.room.QueueRoomContextualGuidanceInput(ctx, sessionKey, roundID, contextName, content, excludedAgentID, objectiveRevision)
	}
	return d.runtime.QueueContextualGuidanceInput(ctx, sessionKey, roundID, contextName, content)
}

type goalInterruptDM interface {
	HandleInterrupt(context.Context, dmsvc.InterruptRequest) error
}

type goalInterruptRoom interface {
	HandleInterrupt(context.Context, roomrealtime.InterruptRequest) error
}

type goalInterruptDispatcher struct {
	dm   goalInterruptDM
	room goalInterruptRoom
}

// NewInterruptDispatcher 创建按 Session 类型路由的 Goal 中断器。
func NewInterruptDispatcher(dm *dmsvc.Service, room *roomrealtime.Service) *goalInterruptDispatcher {
	return &goalInterruptDispatcher{dm: dm, room: room}
}

func (d *goalInterruptDispatcher) InterruptGoalRuntime(ctx context.Context, sessionKey string, roundIDs []string) error {
	if d == nil {
		return errors.New("goal runtime interrupter is not configured")
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	normalizedRoundIDs := make([]string, 0, len(roundIDs))
	for _, roundID := range roundIDs {
		if roundID = strings.TrimSpace(roundID); roundID != "" {
			normalizedRoundIDs = append(normalizedRoundIDs, roundID)
		}
	}
	if len(normalizedRoundIDs) == 0 {
		// 缺少精确 round 时不能把 Goal pause 扩大为整个 Session 中断。
		return nil
	}
	var interruptErrors []error
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if d.room == nil {
			return nil
		}
		for _, roundID := range normalizedRoundIDs {
			if err := d.room.HandleInterrupt(ctx, roomrealtime.InterruptRequest{SessionKey: sessionKey, AgentRoundID: roundID}); err != nil {
				interruptErrors = append(interruptErrors, err)
			}
		}
		return errors.Join(interruptErrors...)
	case protocol.SessionKeyKindAgent:
		if parsed.ChatType == "group" && strings.TrimSpace(parsed.Ref) != "" {
			if d.room == nil {
				return nil
			}
			for _, roundID := range normalizedRoundIDs {
				if err := d.room.HandleInterrupt(ctx, roomrealtime.InterruptRequest{
					SessionKey:   protocol.BuildRoomSharedSessionKey(parsed.Ref),
					AgentRoundID: roundID,
				}); err != nil {
					interruptErrors = append(interruptErrors, err)
				}
			}
			return errors.Join(interruptErrors...)
		}
		if d.dm == nil {
			return nil
		}
		for _, roundID := range normalizedRoundIDs {
			if err := d.dm.HandleInterrupt(ctx, dmsvc.InterruptRequest{SessionKey: sessionKey, RoundID: roundID}); err != nil {
				interruptErrors = append(interruptErrors, err)
			}
		}
		return errors.Join(interruptErrors...)
	default:
		return nil
	}
}

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

// NewContinuationDispatcher 创建 Goal durable resume 的运行时派发器。
func NewContinuationDispatcher(runtime *runtimectx.Manager, dm *dmsvc.Service, room *roomrealtime.Service) *goalContinuationDispatcher {
	return &goalContinuationDispatcher{runtime: runtime, dm: dm, room: room}
}

func (d *goalContinuationDispatcher) ShouldDeferGoalContinuation(ctx context.Context, sessionKey string) bool {
	sessionKey = strings.TrimSpace(sessionKey)
	if d == nil || sessionKey == "" {
		return true
	}
	if d.runtime != nil {
		if len(d.runtime.GetRunningRoundIDs(sessionKey)) > 0 {
			return true
		}
		// terminal 事件可能早于 Goal usage/result 收尾，create guard 仍在时不能并发续跑。
		if len(d.runtime.GoalAccountingCreateConflicts(sessionKey, "")) > 0 {
			return true
		}
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
