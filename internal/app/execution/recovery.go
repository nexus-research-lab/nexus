// INPUT: Authoritative Execution identity and a stable recovery receipt key.
// OUTPUT: Scope-specific durable coordination input and read-only recovery receipts.
// POS: App-only routing between orchestration and DM/Room services.
package execution

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type recoveryQueue interface {
	QueueExecutionRecovery(context.Context, protocol.Execution, string) error
	QueueWorkRecovery(context.Context, protocol.Execution, protocol.ExecutionRecoveryTarget, string) error
	FindWorkRecovery(context.Context, protocol.Execution, string) (*protocol.ExecutionRecoveryTarget, error)
}

func (c RecoveryConsumer) QueueWorkRecovery(ctx context.Context, e protocol.Execution, target protocol.ExecutionRecoveryTarget, key string) error {
	if e.ScopeKind == protocol.ExecutionScopeRoom {
		return c.Room.QueueWorkRecovery(ctx, e, target, key)
	}
	return c.DM.QueueWorkRecovery(ctx, e, target, key)
}

type RecoveryConsumer struct{ DM, Room recoveryQueue }

func (c RecoveryConsumer) FindWorkRecovery(ctx context.Context, e protocol.Execution, key string) (*protocol.ExecutionRecoveryTarget, error) {
	if e.ScopeKind == protocol.ExecutionScopeRoom {
		return c.Room.FindWorkRecovery(ctx, e, key)
	}
	return c.DM.FindWorkRecovery(ctx, e, key)
}

func (c RecoveryConsumer) QueueExecutionRecovery(ctx context.Context, e protocol.Execution, key string) error {
	if e.ScopeKind == protocol.ExecutionScopeRoom {
		return c.Room.QueueExecutionRecovery(ctx, e, key)
	}
	return c.DM.QueueExecutionRecovery(ctx, e, key)
}
