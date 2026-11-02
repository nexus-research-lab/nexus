// INPUT: Authoritative Execution and queue recovery identity.
// OUTPUT: Reconciliation-only coordinator instructions and stale-work checks.
// POS: Shared DM/Room recovery input semantics; no automatic tool replay.
package runtimehost

import (
	"context"
	"errors"
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func ExecutionRecoveryInstruction(e protocol.Execution) string {
	return fmt.Sprintf("Host recovery for Execution %s. Inspect its authoritative current state, accepted work and pending submissions. A previous runtime ended or the host restarted; tool outcomes may be unknown. Reconcile files, sent messages and external side effects before any retry. Preserve accepted work. Use take_over_work only after reconciliation when a root Attempt failed or was interrupted. Continue coordinating the existing objective; do not create a replacement Execution or blindly replay tools.", e.ID)
}

type recoveryExecutionValidator interface {
	RecoveryExecutionCurrent(context.Context, string, string, string) (bool, error)
}

func (h *Host) RecoveryInputCurrent(ctx context.Context, item protocol.InputQueueItem) (bool, error) {
	sessionKey := item.SessionKey
	if item.Scope == protocol.InputQueueScopeRoom {
		sessionKey = protocol.BuildRoomSharedSessionKey(item.ConversationID)
	}
	if item.WorkBinding != nil {
		if validator, ok := h.ExecutionContext.(interface {
			RecoveryWorkInputCurrent(context.Context, string, string, protocol.ExecutionWorkBinding) (bool, error)
		}); ok {
			return validator.RecoveryWorkInputCurrent(ctx, item.OwnerUserID, sessionKey, *item.WorkBinding)
		}
	}
	if item.Source != protocol.InputQueueSourceHostRecovery {
		return true, nil
	}
	validator, ok := h.ExecutionContext.(recoveryExecutionValidator)
	if !ok {
		return false, errors.New("execution recovery validator unavailable")
	}
	return validator.RecoveryExecutionCurrent(ctx, item.OwnerUserID, sessionKey, item.RecoveryExecutionID)
}

// VisitInputQueues scans only active owners' canonical workspaces. Services
// retain authority checks and serial dispatch admission inside the callback.
func (h *Host) VisitInputQueues(ctx context.Context, scope protocol.InputQueueScope, visit func(context.Context, workspacestore.InputQueueLocation)) error {
	agents, err := h.Agents.ListAllAgentRecordsForMaintenance(ctx)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if err := ctx.Err(); err != nil {
			return err
		}
		locations, err := h.InputQueue.LocationsForWorkspace(agent.OwnerUserID, agent.WorkspacePath)
		if err != nil {
			return err
		}
		for _, location := range locations {
			if location.Scope == scope {
				visit(ContextWithExactOwner(ctx, agent.OwnerUserID), location)
			}
		}
	}
	return nil
}

type queueRecoveryKey struct{}

// WithQueueRecovery distinguishes an unresolved prior dispatch from fresh input.
func WithQueueRecovery(ctx context.Context, pending bool) context.Context {
	return context.WithValue(ctx, queueRecoveryKey{}, pending)
}

// QueueRecoveryInputs prevents resuming a surviving dispatch claim as a blind send.
func QueueRecoveryInputs(ctx context.Context) []runtimectx.ContextualInputBlock {
	if ctx == nil {
		return nil
	}
	pending, _ := ctx.Value(queueRecoveryKey{}).(bool)
	if !pending {
		return nil
	}
	return []runtimectx.ContextualInputBlock{runtimectx.NewContextualInputBlock(
		runtimectx.ContextualInputNameRoundRecovery,
		"The host recovered this queued input after an interrupted dispatch. Its previous execution outcome is unknown. Inspect existing transcript, files, messages and authoritative receipts before proceeding. Preserve completed work and do not replay side effects merely because a result is missing.",
		runtimectx.ContextualInputPriorityRoundRecovery, nil,
	)}
}

// DeferRecoveryDispatch makes startup settlement complete before queue admission.
type deferRecoveryDispatchKey struct{}

func DeferRecoveryDispatch(ctx context.Context) context.Context {
	return context.WithValue(ctx, deferRecoveryDispatchKey{}, true)
}
func RecoveryDispatchDeferred(ctx context.Context) bool {
	value, _ := ctx.Value(deferRecoveryDispatchKey{}).(bool)
	return value
}
